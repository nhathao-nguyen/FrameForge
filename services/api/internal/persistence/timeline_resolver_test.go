package persistence

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDurableTimelineResolverChecksOwnershipAndUsableStates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	resolver := DurableTimelineReferenceResolver{SQL: &SQLStore{DB: db}, Workspace: "ws_1"}

	assetQuery := regexp.QuoteMeta("SELECT COALESCE(a.duration_sec, 0)::double precision, COALESCE(a.width, 0), COALESCE(a.height, 0)")
	mock.ExpectQuery(assetQuery).WithArgs("asset_1", "project_1", "ws_1", "artifact_1").WillReturnRows(sqlmock.NewRows([]string{"duration", "width", "height"}).AddRow(12.5, 1920, 1080))
	ready, duration, width, height, err := resolver.ResolveAssetArtifact("asset_1", "artifact_1", "project_1")
	if err != nil || !ready || duration != 12.5 || width != 1920 || height != 1080 {
		t.Fatalf("valid durable asset/artifact was not resolved: ready=%v duration=%v size=%dx%d err=%v", ready, duration, width, height, err)
	}

	// The same ownership/state predicate rejects a foreign Project, Workspace,
	// missing Artifact, staging Artifact, or non-ready Asset as no usable ref.
	mock.ExpectQuery(assetQuery).WithArgs("asset_foreign", "project_1", "ws_1", "artifact_1").WillReturnError(errors.New("database unavailable"))
	if _, _, _, _, err := resolver.ResolveAssetArtifact("asset_foreign", "artifact_1", "project_1"); err == nil {
		t.Fatal("database resolver converted a durable query failure into success")
	}
	mock.ExpectQuery(assetQuery).WithArgs("asset_1", "project_1", "ws_1", "artifact_missing").WillReturnRows(sqlmock.NewRows([]string{"duration", "width", "height"}))
	ready, _, _, _, err = resolver.ResolveAssetArtifact("asset_1", "artifact_missing", "project_1")
	if err != nil || ready {
		t.Fatalf("missing durable artifact was accepted: ready=%v err=%v", ready, err)
	}

	sceneQuery := regexp.QuoteMeta("SELECT s.source_start_sec::double precision, s.source_end_sec::double precision FROM scenes s")
	mock.ExpectQuery(sceneQuery).WithArgs("scene_1", "project_1", "ws_1", "asset_1", "artifact_1").WillReturnRows(sqlmock.NewRows([]string{"start", "end"}).AddRow(2.0, 6.0))
	ready, start, end, err := resolver.ResolveScene("scene_1", "asset_1", "artifact_1", "project_1")
	if err != nil || !ready || start != 2 || end != 6 {
		t.Fatalf("valid scene reference was not resolved: ready=%v range=%v..%v err=%v", ready, start, end, err)
	}
	mock.ExpectQuery(sceneQuery).WithArgs("scene_1", "project_1", "ws_1", "asset_1", "artifact_wrong").WillReturnRows(sqlmock.NewRows([]string{"start", "end"}))
	ready, _, _, err = resolver.ResolveScene("scene_1", "asset_1", "artifact_wrong", "project_1")
	if err != nil || ready {
		t.Fatalf("foreign/mismatched scene source was accepted: ready=%v err=%v", ready, err)
	}

	artifactQuery := regexp.QuoteMeta("SELECT 1 FROM artifacts ar")
	mock.ExpectQuery(artifactQuery).WithArgs("artifact_1", "project_1", "ws_1").WillReturnRows(sqlmock.NewRows([]string{"present"}).AddRow(1))
	committed, err := resolver.ResolveArtifact("artifact_1", "project_1")
	if err != nil || !committed {
		t.Fatalf("committed artifact was not resolved: committed=%v err=%v", committed, err)
	}
	mock.ExpectQuery(artifactQuery).WithArgs("artifact_staging", "project_1", "ws_1").WillReturnRows(sqlmock.NewRows([]string{"present"}))
	committed, err = resolver.ResolveArtifact("artifact_staging", "project_1")
	if err != nil || committed {
		t.Fatalf("uncommitted artifact was accepted: committed=%v err=%v", committed, err)
	}

	narrationQuery := regexp.QuoteMeta("SELECT 1 FROM narrations n")
	mock.ExpectQuery(narrationQuery).WithArgs("narration_1", "project_1", "ws_1", "scriptv_1").WillReturnRows(sqlmock.NewRows([]string{"present"}).AddRow(1))
	ready, err = resolver.ResolveNarration("narration_1", "scriptv_1", "project_1")
	if err != nil || !ready {
		t.Fatalf("valid narration/script/artifact was not resolved: ready=%v err=%v", ready, err)
	}
	mock.ExpectQuery(narrationQuery).WithArgs("narration_1", "project_1", "ws_1", "scriptv_foreign").WillReturnRows(sqlmock.NewRows([]string{"present"}))
	ready, err = resolver.ResolveNarration("narration_1", "scriptv_foreign", "project_1")
	if err != nil || ready {
		t.Fatalf("foreign narration content was accepted: ready=%v err=%v", ready, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
