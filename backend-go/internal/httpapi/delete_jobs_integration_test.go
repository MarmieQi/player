package httpapi_test

import (
	"context"
	"testing"
	"time"

	"player-stats-backend-go/internal/importer"
)

func TestDeleteJobHTTPAcceptsBeforeWriterCompletes(t *testing.T) {
	f := newPublicFixture(t)
	f.request(t, "POST", "/api/import/delete-jobs", map[string]any{"files": []any{}}, "", 401, nil)
	f.request(t, "POST", "/api/import/delete-jobs", map[string]any{"files": []any{}}, "admin", 400, nil)
	writer, err := f.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	var job importer.DeleteJobView
	f.request(t, "POST", "/api/import/delete-jobs", map[string]any{"files": []map[string]string{{"serverId": "main", "remotePath": "player_actions_2026-04-01.csv"}}}, "admin", 202, &job)
	if job.JobID == "" || job.DeletedFiles != 0 {
		t.Fatal(job)
	}
	f.request(t, "GET", "/api/import/delete-jobs/"+job.JobID, nil, "admin", 200, &job)
	if job.DeletedFiles != 0 {
		t.Fatal("reported uncommitted success")
	}
	if err := writer.Rollback(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		f.request(t, "GET", "/api/import/delete-jobs/"+job.JobID, nil, "admin", 200, &job)
		if job.Status != "PENDING" && job.Status != "RUNNING" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job timed out")
		}
		time.Sleep(time.Millisecond)
	}
	if job.Status != "FINISHED" || job.DeletedFiles != 1 {
		t.Fatal(job)
	}
}
