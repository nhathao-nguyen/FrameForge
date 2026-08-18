package persistence

import (
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/product"
)

func expectDurableRenderTimelinePrerequisites(mock sqlmock.Sqlmock, timelineStatus string, document []byte) {
	now := time.Now().UTC()
	for i := 0; i < 2; i++ {
		mock.ExpectQuery("SELECT wm.role FROM workspace_members").WithArgs("ws_1", "user_1").WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow("owner"))
		mock.ExpectQuery("SELECT id::text,workspace_id::text,name").WithArgs("project_1", "ws_1").WillReturnRows(sqlmock.NewRows([]string{"id", "workspace_id", "name", "workflow_key", "status", "settings", "revision", "created_at", "updated_at"}).AddRow("project_1", "ws_1", "Project", "movie_recap", "active", []byte(`{}`), int64(1), now, now))
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT v.id::text,v.timeline_id::text,t.project_id::text,v.version,v.status,v.schema_version,v.document,v.content_hash,v.origin,COALESCE(v.based_on_version_id::text,''),v.created_at")).WithArgs("timeline_version_1", "project_1").WillReturnRows(sqlmock.NewRows([]string{"id", "timeline_id", "project_id", "version", "status", "schema_version", "document", "content_hash", "origin", "based_on", "created_at"}).AddRow("timeline_version_1", "timeline_1", "project_1", 1, timelineStatus, "1.0", document, "hash", "user", "", now))
	mock.ExpectQuery("SELECT status FROM timeline_versions").WithArgs("timeline_version_1", "project_1").WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(timelineStatus))
}

func TestDurableCreateRenderFailsWithoutExistingProfileAndNeverAutoCreates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	backend, err := NewDurableBackend(&SQLStore{DB: db}, "user_1", "ws_1", nil)
	if err != nil {
		t.Fatal(err)
	}
	document, _ := json.Marshal(map[string]any{
		"schema_version": "1.0", "timeline_id": "tl_client", "timeline_version_id": "tlv_client", "project_id": "project_1", "version": 1, "duration_sec": 10,
		"tracks": []any{map[string]any{"id": "track", "kind": "video", "name": "Footage", "order": 0, "clips": []any{map[string]any{"id": "clip", "timeline_in_sec": 0, "timeline_out_sec": 1, "source": map[string]any{"type": "none", "inline_id": "silence"}, "origin": "user"}}}},
	})
	expectDurableRenderTimelinePrerequisites(mock, "approved", document)
	mock.ExpectQuery("SELECT id::text,status,document,content_hash FROM render_profiles").WithArgs("ws_1", "missing", 1).WillReturnError(sql.ErrNoRows)
	_, err = backend.CreateRender("ws_1", "project_1", "timeline_version_1", "missing", 1, map[string]any{"preview": false})
	if !errors.Is(err, product.ErrNotFound) {
		t.Fatalf("nonexistent profile did not fail explicitly: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDurableCreateRenderRejectsDraftProfileBeforeRenderInsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	backend, err := NewDurableBackend(&SQLStore{DB: db}, "user_1", "ws_1", nil)
	if err != nil {
		t.Fatal(err)
	}
	document := []byte(`{"schema_version":"1.0","timeline_id":"tl_client","timeline_version_id":"tlv_client","project_id":"project_1","version":1,"duration_sec":10,"tracks":[{"id":"track","kind":"video","name":"Footage","order":0,"clips":[{"id":"clip","timeline_in_sec":0,"timeline_out_sec":1,"source":{"type":"none","inline_id":"silence"},"origin":"user"}]}]}`)
	expectDurableRenderTimelinePrerequisites(mock, "approved", document)
	mock.ExpectQuery("SELECT id::text,status,document,content_hash FROM render_profiles").WithArgs("ws_1", "draft", 1).WillReturnRows(sqlmock.NewRows([]string{"id", "status", "document", "content_hash"}).AddRow("profile_1", "draft", []byte(`{"width":640}`), "hash"))
	_, err = backend.CreateRender("ws_1", "project_1", "timeline_version_1", "draft", 1, nil)
	if !errors.Is(err, product.ErrConflict) {
		t.Fatalf("draft profile was eligible for new render: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
