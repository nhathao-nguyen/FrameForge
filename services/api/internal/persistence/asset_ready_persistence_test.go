package persistence

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func expectAssetStatusProjectScope(mock sqlmock.Sqlmock) {
	now := time.Now().UTC()
	mock.ExpectQuery("SELECT wm.role FROM workspace_members").
		WithArgs("ws_1", "user_1").
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow("owner"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text,workspace_id::text,name,(SELECT workflow_key FROM workflows WHERE id=projects.workflow_id),status,settings,revision,created_at,updated_at FROM projects WHERE id=$1 AND workspace_id=$2 AND deleted_at IS NULL")).
		WithArgs("project_1", "ws_1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "workspace_id", "name", "workflow_key", "status", "settings", "revision", "created_at", "updated_at"}).
			AddRow("project_1", "ws_1", "Project", "movie_recap", "active", []byte(`{}`), int64(1), now, now))
}

func TestSQLStoreSetAssetStatusRejectsUncommittedArtifact(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := &SQLStore{DB: db}
	expectAssetStatusProjectScope(mock)
	mock.ExpectQuery("UPDATE assets SET status=\\$3").
		WithArgs("asset_1", "project_1", "ready", "artifact_staging", "user_1").
		WillReturnError(sql.ErrNoRows)
	_, err = store.SetAssetStatus(context.Background(), "user_1", "ws_1", "project_1", "asset_1", "ready", "artifact_staging")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("uncommitted artifact was allowed to become ready: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
