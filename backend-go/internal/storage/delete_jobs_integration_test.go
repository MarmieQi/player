package storage_test

import (
	"context"
	"os"
	"testing"
	"time"

	"player-stats-backend-go/internal/importer"
	"player-stats-backend-go/internal/settings"
)

func waitDeleteJob(t *testing.T, jobs *importer.JobService, id string) importer.DeleteJobView {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, err := jobs.GetDeleteJob(id)
		if err != nil {
			t.Fatal(err)
		}
		if job.Status != "PENDING" && job.Status != "RUNNING" {
			return job
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("delete job did not finish")
	return importer.DeleteJobView{}
}

func TestDeleteJobSurvivesRequestCancellationAndReportsOnlyCommittedResults(t *testing.T) {
	db, cfg := fixture(t)
	ctx := context.Background()
	svc := importer.NewService(db, cfg, settings.NewService(db, cfg))
	defer svc.CloseLiveLogSnapshots()
	path := writeCSV(t, cfg, "2026-04-01", "Alex", 3)
	if _, err := svc.ImportFromConfiguredSource(ctx, "main", true, nil); err != nil {
		t.Fatal(err)
	}
	jobs := importer.NewJobService(svc)
	defer jobs.Close()
	writer, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	requestCtx, cancel := context.WithCancel(ctx)
	request := importer.DeleteImportRecordsRequest{Files: []importer.ImportRecordKey{{ServerID: "main", RemotePath: path}, {ServerID: "main", RemotePath: path}}}
	job, err := jobs.StartDeleteRecords(requestCtx, request)
	if err != nil {
		t.Fatal(err)
	}
	if job.TotalFiles != 1 || job.DeletedFiles != 0 {
		t.Fatal(job)
	}
	cancel() // The acknowledged background job must outlive this HTTP connection.
	if _, err := jobs.StartImport(ctx, "main", true); err == nil {
		t.Fatal("import overlapped delete job")
	}
	if _, err := jobs.StartDeleteRecords(ctx, request); err == nil {
		t.Fatal("duplicate delete started")
	}
	pending, err := jobs.GetDeleteJob("latest")
	if err != nil || pending.JobID != job.JobID || pending.DeletedFiles != 0 {
		t.Fatalf("pending: %+v %v", pending, err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `select count(*) from import_files`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("uncommitted change leaked: %d %v", count, err)
	}
	if err := writer.Rollback(); err != nil {
		t.Fatal(err)
	}
	finished := waitDeleteJob(t, jobs, job.JobID)
	if finished.Status != "FINISHED" || finished.DeletedFiles != 1 || finished.FailedFiles != 0 {
		t.Fatal(finished)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("original CSV was removed", err)
	}
	if err := db.Check(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestClosingDeleteJobRollsBackAndAllowsRefresh(t *testing.T) {
	db, cfg := fixture(t)
	ctx := context.Background()
	svc := importer.NewService(db, cfg, settings.NewService(db, cfg))
	defer svc.CloseLiveLogSnapshots()
	path := writeCSV(t, cfg, "2026-04-01", "Alex", 3)
	if _, err := svc.ImportFromConfiguredSource(ctx, "main", true, nil); err != nil {
		t.Fatal(err)
	}
	jobs := importer.NewJobService(svc)
	writer, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	job, err := jobs.StartDeleteRecords(ctx, importer.DeleteImportRecordsRequest{Files: []importer.ImportRecordKey{{ServerID: "main", RemotePath: path}}})
	if err != nil {
		t.Fatal(err)
	}
	jobs.Close()
	finished := waitDeleteJob(t, jobs, job.JobID)
	if finished.Status != "FAILED" || finished.DeletedFiles != 0 {
		t.Fatal(finished)
	}
	if err := writer.Rollback(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `select count(*) from import_files`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("cancelled deletion committed: %d %v", count, err)
	}
	restarted := importer.NewJobService(svc)
	defer restarted.Close()
	if _, err := restarted.GetDeleteJob(job.JobID); err == nil {
		t.Fatal("restart retained an invalid in-memory job")
	}
}
