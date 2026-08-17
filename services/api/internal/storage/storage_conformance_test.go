package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
)

// The live test is opt-in so ordinary unit verification does not require a
// storage service. The Gate C+D acceptance run enables it against a private
// local MinIO instance.
func TestS3StorageConformance(t *testing.T) {
	if os.Getenv("NH_MEDIA_S3_CONFORMANCE") != "1" {
		t.Skip("set NH_MEDIA_S3_CONFORMANCE=1 to run live S3/MinIO conformance")
	}
	endpoint := os.Getenv("NH_MEDIA_S3_ENDPOINT")
	accessKey := os.Getenv("NH_MEDIA_S3_ACCESS_KEY")
	secretKey := os.Getenv("NH_MEDIA_S3_SECRET_KEY")
	bucket := os.Getenv("NH_MEDIA_S3_BUCKET")
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		t.Fatal("NH_MEDIA_S3_ENDPOINT, NH_MEDIA_S3_ACCESS_KEY, NH_MEDIA_S3_SECRET_KEY and NH_MEDIA_S3_BUCKET are required")
	}
	store, err := NewS3Storage(endpoint, accessKey, secretKey, bucket, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	exists, err := store.client.Client.BucketExists(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		if err := store.client.Client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	suffix := newRandomID()
	scope := Scope{WorkspaceID: "conformance-ws", ProjectID: "conformance-project"}
	staged, err := store.PutStaged(ctx, scope, strings.NewReader("0123456789"), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Delete(ctx, staged.Locator, Preconditions{})
	expected := sha256.Sum256([]byte("0123456789"))
	if staged.SHA256 != hex.EncodeToString(expected[:]) {
		t.Fatalf("staged checksum mismatch: %s", staged.SHA256)
	}
	finalKey := "workspaces/conformance-ws/projects/conformance-project/artifacts/conformance-" + suffix
	metadata, err := store.Promote(ctx, staged, finalKey, Preconditions{IfNoneMatch: true})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Delete(ctx, metadata.Locator, Preconditions{})
	if _, err := store.Promote(ctx, staged, finalKey, Preconditions{IfNoneMatch: true}); err == nil {
		t.Fatal("If-None-Match promotion precondition was ignored")
	}
	reader, err := store.OpenRead(ctx, metadata.Locator, &ByteRange{Start: 2, End: 5})
	if err != nil {
		t.Fatal(err)
	}
	rangeBytes, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if readErr != nil || string(rangeBytes) != "2345" {
		t.Fatalf("range read: %q %v", rangeBytes, readErr)
	}
	signed, err := store.PresignDownload(ctx, metadata.Locator, time.Minute, "inline")
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(signed.URL)
	if err != nil {
		t.Fatal(err)
	}
	downloaded, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || string(downloaded) != "0123456789" {
		t.Fatalf("presigned download: status=%d body=%q err=%v", response.StatusCode, downloaded, readErr)
	}
	anonymousURL := endpointURL(endpoint, bucket, metadata.Locator.ObjectKey)
	anonymous, err := http.Get(anonymousURL)
	if err != nil {
		t.Fatal(err)
	}
	_ = anonymous.Body.Close()
	if anonymous.StatusCode == http.StatusOK {
		t.Fatal("private object was anonymously readable")
	}

	multipart, err := store.InitiateUpload(ctx, scope, UploadConstraints{Multipart: true, ContentType: "application/octet-stream", PartSize: 5 << 20})
	if err != nil {
		t.Fatal(err)
	}
	partBytes := bytes.Repeat([]byte("m"), 5<<20)
	partRequest, err := store.PresignPart(ctx, multipart, 1)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, partRequest.URL, bytes.NewReader(partBytes))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", partRequest.Headers["Content-Type"])
	partResponse, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = partResponse.Body.Close()
	if partResponse.StatusCode < 200 || partResponse.StatusCode >= 300 {
		t.Fatalf("multipart upload: status=%d", partResponse.StatusCode)
	}
	etag := partResponse.Header.Get("ETag")
	if etag == "" {
		t.Fatal("multipart upload did not return an ETag")
	}
	multipartObject, err := store.CompleteUpload(ctx, multipart, []UploadedPart{{PartNumber: 1, ETag: etag}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Delete(ctx, multipartObject.Locator, Preconditions{})
	if multipartObject.SizeBytes != int64(len(partBytes)) {
		t.Fatalf("multipart size: got %d want %d", multipartObject.SizeBytes, len(partBytes))
	}

	expired, err := store.InitiateUpload(ctx, scope, UploadConstraints{Multipart: true, ExpiresAt: time.Now().UTC().Add(-time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PresignPart(ctx, expired, 1); err == nil {
		t.Fatal("expired multipart session was presigned")
	}
	if err := store.AbortUpload(ctx, expired); err != nil {
		t.Fatal(err)
	}
}

func endpointURL(endpoint, bucket, objectKey string) string {
	return (&url.URL{Scheme: "http", Host: endpoint, Path: "/" + bucket + "/" + objectKey}).String()
}
