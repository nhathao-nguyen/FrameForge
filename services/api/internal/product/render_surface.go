package product

import (
	"sort"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/domain"
)

func (s *Store) GetRender(workspaceID, projectID, renderID string) (*Render, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.renders[renderID]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	return cloneRender(value), nil
}

func (s *Store) ListRenders(workspaceID, projectID, status, timelineVersionID string, limit int) ([]Render, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	values := make([]Render, 0)
	for _, value := range s.renders {
		if value.ProjectID != projectID || (status != "" && value.Status != status) || (timelineVersionID != "" && value.TimelineVersionID != timelineVersionID) {
			continue
		}
		values = append(values, *cloneRender(value))
	}
	sort.Slice(values, func(i, j int) bool { return values[i].CreatedAt.After(values[j].CreatedAt) })
	if len(values) > limit {
		values = values[:limit]
	}
	return values, nil
}

func (s *Store) CancelRender(workspaceID, projectID, renderID string) (*Render, error) {
	value, err := s.GetRender(workspaceID, projectID, renderID)
	if err != nil {
		return nil, err
	}
	job, err := s.CancelJob(workspaceID, projectID, value.JobID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if stored := s.renders[renderID]; stored != nil && job.Status == domain.JobCancelled {
		stored.Status = "cancelled"
	}
	value = cloneRender(s.renders[renderID])
	s.mu.Unlock()
	return value, nil
}

func (s *Store) RetryRender(workspaceID, projectID, renderID string, autoStart bool) (*Render, error) {
	old, err := s.GetRender(workspaceID, projectID, renderID)
	if err != nil {
		return nil, err
	}
	if old.Status != "failed" && old.Status != "cancelled" {
		return nil, ErrConflict
	}
	job, err := s.RetryExecutionJob(workspaceID, projectID, old.JobID, autoStart)
	if err != nil {
		return nil, err
	}
	value := &Render{ID: newID("render"), ProjectID: projectID, TimelineVersionID: old.TimelineVersionID, ProfileKey: old.ProfileKey, ProfileVersion: old.ProfileVersion, ProfileID: old.ProfileID, ProfileSnapshot: copyMap(old.ProfileSnapshot), Status: "created", JobID: job.ID, SupersedesRenderID: old.ID, RequestHash: old.RequestHash, CreatedAt: now()}
	s.mu.Lock()
	s.renders[value.ID] = value
	if stored := s.jobs[job.ID]; stored != nil {
		stored.Command["render_id"] = value.ID
		stored.Command["supersedes_render_id"] = old.ID
	}
	s.mu.Unlock()
	return cloneRender(value), nil
}
