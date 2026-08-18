package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
)

func TestTransitionAtomicallyUpdatesJobAndAppendsEventOutbox(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewSQLStore(db)
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE jobs j SET status=").WithArgs("ws_1", "project_1", "job_1", "queued", "created").
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow("job_1", "queued"))
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock(hashtextextended($1,0))")).WithArgs("job_1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(sequence),0)+1 FROM job_events WHERE job_id=$1::uuid")).WithArgs("job_1").
		WillReturnRows(sqlmock.NewRows([]string{"sequence"}).AddRow(int64(4)))
	mock.ExpectQuery(`INSERT INTO job_events\(schema_version,workspace_id,project_id,job_id`).
		WithArgs("1.0", "ws_1", "project_1", "job_1", "", "", "", "", int64(4), "job.queued", sqlmock.AnyArg(), "corr_1").
		WillReturnRows(sqlmock.NewRows([]string{"event_id"}).AddRow("event_1"))
	mock.ExpectExec(`INSERT INTO outbox_events\(id,aggregate_type,aggregate_id,event_type,payload,status\)`).
		WithArgs("event_1", "job_1", "job.queued", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := store.Transition(context.Background(), TransitionInput{
		Entity:        execution.EntityJob,
		WorkspaceID:   "ws_1",
		ProjectID:     "project_1",
		JobID:         "job_1",
		FromStatus:    "created",
		ToStatus:      "queued",
		CorrelationID: "corr_1",
		Payload:       json.RawMessage(`{"priority":5}`),
	})
	if err != nil {
		t.Fatalf("transition failed: %v", err)
	}
	if result.Status != "queued" || result.Sequence != 4 || result.EventType != "job.queued" {
		t.Fatalf("unexpected transition result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTransitionRejectsInvalidStateBeforeOpeningDatabase(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, _ := NewSQLStore(db)
	_, err = store.Transition(context.Background(), TransitionInput{
		Entity: execution.EntityJob, WorkspaceID: "ws_1", ProjectID: "project_1", JobID: "job_1",
		FromStatus: "created", ToStatus: "running",
	})
	if !errors.Is(err, execution.ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTransitionMapsConcurrentStatusLossToConflictAndRollsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, _ := NewSQLStore(db)
	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE jobs j SET status=").WithArgs("ws_1", "project_1", "job_1", "queued", "created").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, err = store.Transition(context.Background(), TransitionInput{
		Entity: execution.EntityJob, WorkspaceID: "ws_1", ProjectID: "project_1", JobID: "job_1",
		FromStatus: "created", ToStatus: "queued",
	})
	if !errors.Is(err, ErrTransitionConflict) {
		t.Fatalf("expected transition conflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
