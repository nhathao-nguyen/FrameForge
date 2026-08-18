package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/domain"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/product"
)

func (b *DurableBackend) GetRender(workspaceID, projectID, renderID string) (*product.Render, error) {
	if b == nil || b.SQL == nil || workspaceID != b.Workspace {
		return nil, product.ErrNotFound
	}
	value, err := scanRender(b.SQL.DB.QueryRowContext(context.Background(), `SELECT r.id::text,r.project_id::text,r.timeline_version_id::text,r.render_profile_id::text,rp.profile_key,rp.version,r.status,COALESCE(r.job_id::text,''),COALESCE(r.supersedes_render_id::text,''),COALESCE(r.request_hash,''),r.profile_snapshot,r.created_at FROM renders r JOIN render_profiles rp ON rp.id=r.render_profile_id JOIN projects p ON p.id=r.project_id WHERE p.workspace_id=$1::uuid AND r.project_id=$2::uuid AND r.id=$3::uuid`, workspaceID, projectID, renderID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, product.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (b *DurableBackend) ListRenders(workspaceID, projectID, status, timelineVersionID string, limit int) ([]product.Render, error) {
	if b == nil || b.SQL == nil || workspaceID != b.Workspace {
		return nil, product.ErrNotFound
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args := []any{workspaceID, projectID}
	query := `SELECT r.id::text,r.project_id::text,r.timeline_version_id::text,r.render_profile_id::text,rp.profile_key,rp.version,r.status,COALESCE(r.job_id::text,''),COALESCE(r.supersedes_render_id::text,''),COALESCE(r.request_hash,''),r.profile_snapshot,r.created_at FROM renders r JOIN render_profiles rp ON rp.id=r.render_profile_id JOIN projects p ON p.id=r.project_id WHERE p.workspace_id=$1::uuid AND r.project_id=$2::uuid`
	if status != "" {
		args = append(args, status)
		query += fmt.Sprintf(" AND r.status=$%d", len(args))
	}
	if timelineVersionID != "" {
		args = append(args, timelineVersionID)
		query += fmt.Sprintf(" AND r.timeline_version_id=$%d::uuid", len(args))
	}
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY r.created_at DESC,r.id LIMIT $%d", len(args))
	rows, err := b.SQL.DB.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]product.Render, 0)
	for rows.Next() {
		value, err := scanRender(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *value)
	}
	return result, rows.Err()
}

func (b *DurableBackend) CancelRender(workspaceID, projectID, renderID string) (*product.Render, error) {
	value, err := b.GetRender(workspaceID, projectID, renderID)
	if err != nil {
		return nil, err
	}
	job, err := b.CancelJob(workspaceID, projectID, value.JobID)
	if err != nil {
		return nil, err
	}
	if job.Status == domain.JobCancelled {
		if _, err := b.SQL.DB.ExecContext(context.Background(), `UPDATE renders SET status='cancelled',completed_at=COALESCE(completed_at,now()),updated_at=now() WHERE id=$1::uuid AND project_id=$2::uuid AND status NOT IN ('completed','failed','cancelled')`, renderID, projectID); err != nil {
			return nil, err
		}
	}
	return b.GetRender(workspaceID, projectID, renderID)
}

func (b *DurableBackend) RetryRender(workspaceID, projectID, renderID string, autoStart bool) (*product.Render, error) {
	old, err := b.GetRender(workspaceID, projectID, renderID)
	if err != nil {
		return nil, err
	}
	if old.Status != "failed" && old.Status != "cancelled" {
		return nil, product.ErrConflict
	}
	oldJob, err := b.GetJob(workspaceID, projectID, old.JobID)
	if err != nil {
		return nil, err
	}
	command := cloneMap(oldJob.Command)
	command["supersedes_render_id"] = old.ID
	command["render_id"] = ""
	job, err := b.createRetryJob(workspaceID, projectID, oldJob, command)
	if err != nil {
		return nil, err
	}
	var value product.Render
	var profileSnapshot []byte
	if err := b.SQL.DB.QueryRowContext(context.Background(), `INSERT INTO renders(project_id,timeline_version_id,render_profile_id,supersedes_render_id,job_id,profile_snapshot,status,request_hash,request,created_by) SELECT project_id,timeline_version_id,render_profile_id,$4::uuid,$3::uuid,profile_snapshot,'created',request_hash,request,$5::uuid FROM renders WHERE id=$1::uuid AND project_id=$2::uuid RETURNING id::text,project_id::text,timeline_version_id::text,status,created_at,profile_snapshot`, old.ID, projectID, job.ID, old.ID, b.UserID).Scan(&value.ID, &value.ProjectID, &value.TimelineVersionID, &value.Status, &value.CreatedAt, &profileSnapshot); err != nil {
		return nil, err
	}
	value.ProfileKey, value.ProfileVersion, value.ProfileID = old.ProfileKey, old.ProfileVersion, old.ProfileID
	value.ProfileSnapshot = mapFromJSON(profileSnapshot)
	value.JobID = job.ID
	value.SupersedesRenderID = old.ID
	value.RequestHash = old.RequestHash
	command["render_id"] = value.ID
	if _, err := b.SQL.DB.ExecContext(context.Background(), `UPDATE jobs SET command=$2,updated_at=now() WHERE id=$1::uuid`, job.ID, jsonBytes(command)); err != nil {
		return nil, err
	}
	if autoStart {
		if _, err := b.StartJob(workspaceID, projectID, job.ID); err != nil {
			return nil, err
		}
	}
	return &value, nil
}

func (b *DurableBackend) createRetryJob(workspaceID, projectID string, old *product.Job, command map[string]any) (*product.Job, error) {
	record, err := b.SQL.CreateProjectJob(context.Background(), b.UserID, b.Workspace, projectID, old.Kind, jsonBytes(command), jsonBytes(command))
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	return jobFromRecord(record), nil
}

type renderScanner interface {
	Scan(...any) error
}

func scanRender(scanner renderScanner) (*product.Render, error) {
	value := &product.Render{}
	var snapshot []byte
	if err := scanner.Scan(&value.ID, &value.ProjectID, &value.TimelineVersionID, &value.ProfileID, &value.ProfileKey, &value.ProfileVersion, &value.Status, &value.JobID, &value.SupersedesRenderID, &value.RequestHash, &snapshot, &value.CreatedAt); err != nil {
		return nil, err
	}
	if len(snapshot) == 0 {
		snapshot = []byte(`{}`)
	}
	value.ProfileSnapshot = mapFromJSON(snapshot)
	return value, nil
}
