package executor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
)

type transferRequest struct {
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers"`
	ExpiresAt  time.Time         `json:"expires_at"`
	PartNumber int               `json:"part_number,omitempty"`
}

type artifactClient struct {
	apiURL    string
	token     string
	localRoot string
	http      *http.Client
}

func newArtifactClient(apiURL, token, localRoot string) (*artifactClient, error) {
	apiURL = strings.TrimRight(strings.TrimSpace(apiURL), "/")
	if !(strings.HasPrefix(apiURL, "http://") || strings.HasPrefix(apiURL, "https://")) || len(token) < 24 {
		return nil, errors.New("worker Artifact transfer configuration is invalid")
	}
	if localRoot != "" {
		absolute, err := filepath.Abs(localRoot)
		if err != nil {
			return nil, err
		}
		localRoot = absolute
	}
	return &artifactClient{apiURL: apiURL, token: token, localRoot: localRoot, http: &http.Client{Timeout: 5 * time.Minute}}, nil
}

func (c *artifactClient) materialize(ctx context.Context, command worker.Command, ref worker.ArtifactRef, root string) (string, error) {
	transfer, err := c.post(ctx, "/internal/v1/worker/artifacts/resolve", map[string]any{
		"project_id": command.ProjectID, "artifact_id": ref.ArtifactID, "sha256": ref.SHA256,
	})
	if err != nil {
		return "", fmt.Errorf("resolve Artifact transfer: %w", err)
	}
	path := filepath.Join(root, "input-"+safeName(ref.Role)+"-"+ref.SHA256[:12]+extensionForRole(ref.Role))
	if err := c.download(ctx, transfer, path); err != nil {
		return "", fmt.Errorf("download Artifact transfer: %w", err)
	}
	digest, _, err := fileDigest(path)
	if err != nil || digest != ref.SHA256 {
		return "", errors.New("materialized Artifact checksum mismatch")
	}
	return path, nil
}

func (c *artifactClient) stageFile(ctx context.Context, command worker.Command, kind, role, path, contentType string) (worker.OutputRef, error) {
	digest, size, err := fileDigest(path)
	if err != nil {
		return worker.OutputRef{}, err
	}
	artifactID := "artifact_" + safeName(role) + "_" + digest[:16]
	transfer, err := c.post(ctx, "/internal/v1/worker/artifacts/stage", map[string]any{
		"workspace_id": command.WorkspaceID, "project_id": command.ProjectID, "job_id": command.JobID,
		"job_step_id": command.JobStepID, "message_id": command.MessageID, "artifact_id": artifactID,
		"content_type": contentType,
	})
	if err != nil {
		return worker.OutputRef{}, fmt.Errorf("prepare Artifact stage: %w", err)
	}
	if err := c.upload(ctx, transfer, path, size); err != nil {
		return worker.OutputRef{}, fmt.Errorf("upload Artifact stage: %w", err)
	}
	return worker.OutputRef{ArtifactID: artifactID, Kind: kind, Role: role, SHA256: digest, SizeBytes: size, ContentType: contentType}, nil
}

func (c *artifactClient) stageJSON(ctx context.Context, command worker.Command, kind, role string, value any, root string) (worker.OutputRef, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return worker.OutputRef{}, err
	}
	path := filepath.Join(root, "output-"+safeName(role)+".json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return worker.OutputRef{}, err
	}
	return c.stageFile(ctx, command, kind, role, path, "application/json")
}

func (c *artifactClient) post(ctx context.Context, endpoint string, value any) (transferRequest, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return transferRequest{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return transferRequest{}, errors.New("worker Artifact transfer request is invalid")
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return transferRequest{}, errors.New("worker Artifact transfer request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return transferRequest{}, fmt.Errorf("worker Artifact is unavailable (HTTP %d)", response.StatusCode)
	}
	var transfer transferRequest
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&transfer); err != nil || transfer.URL == "" {
		return transferRequest{}, errors.New("worker Artifact transfer response is invalid")
	}
	return transfer, nil
}

func (c *artifactClient) download(ctx context.Context, transfer transferRequest, path string) error {
	if transfer.Method != http.MethodGet {
		return errors.New("worker Artifact download method is invalid")
	}
	if strings.HasPrefix(transfer.URL, "nh-local://") {
		source, err := c.localPath(transfer.URL)
		if err != nil {
			return err
		}
		return copyFile(source, path)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, transfer.URL, nil)
	if err != nil {
		return errors.New("worker Artifact download URL is invalid")
	}
	for key, value := range transfer.Headers {
		request.Header.Set(key, value)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return errors.New("worker object download failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New("worker object download failed")
	}
	output, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, io.LimitReader(response.Body, 8<<30))
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (c *artifactClient) upload(ctx context.Context, transfer transferRequest, path string, size int64) error {
	if transfer.Method != http.MethodPut {
		return errors.New("worker Artifact upload method is invalid")
	}
	if strings.HasPrefix(transfer.URL, "nh-local://") {
		target, err := c.localPath(transfer.URL)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		return copyFile(path, target)
	}
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer input.Close()
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, transfer.URL, input)
	if err != nil {
		return errors.New("worker Artifact upload URL is invalid")
	}
	request.ContentLength = size
	for key, value := range transfer.Headers {
		request.Header.Set(key, value)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return errors.New("worker object upload failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New("worker object upload failed")
	}
	return nil
}

func (c *artifactClient) localPath(raw string) (string, error) {
	if c.localRoot == "" {
		return "", errors.New("local worker storage root is not configured")
	}
	escaped := strings.SplitN(strings.TrimPrefix(raw, "nh-local://"), "?", 2)[0]
	key, err := url.PathUnescape(escaped)
	if err != nil {
		return "", errors.New("local worker Artifact URL is invalid")
	}
	candidate, err := filepath.Abs(filepath.Join(c.localRoot, filepath.FromSlash(key)))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(c.localRoot, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", errors.New("local worker Artifact transfer escaped storage root")
	}
	return candidate, nil
}

func copyFile(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func fileDigest(path string) (string, int64, error) {
	input, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer input.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, input)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(digest.Sum(nil)), size, nil
}

func safeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var result strings.Builder
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_' || char == '-' {
			result.WriteRune(char)
		}
	}
	if result.Len() < 1 {
		return "artifact"
	}
	return result.String()
}

func extensionForRole(role string) string {
	switch role {
	case "source_original", "prepared_video", "render_video", "clip_exports":
		return ".mp4"
	case "prepared_audio", "narration_audio", "mixed_audio", "render_audio":
		return ".wav"
	case "source_thumbnails", "scene_thumbnails":
		return ".jpg"
	case "subtitle_srt":
		return ".srt"
	case "subtitle_vtt":
		return ".vtt"
	case "subtitle_ass":
		return ".ass"
	default:
		return ".json"
	}
}

func checkedRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("worker scratch root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(absolute, 0o700); err != nil {
		return "", err
	}
	return absolute, nil
}

func transferError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("Artifact transfer failed: %w", err)
}
