package importer

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCopyJobsRejectDuplicatesAndIsolateWorkflows(t *testing.T) {
	csv, chat := NewJobService(nil), NewJobService(nil)
	finish := make(chan struct{})
	defer close(finish)
	op := func(ctx context.Context, listener ProgressListener) (ImportRunResult, error) {
		select {
		case <-finish:
		case <-ctx.Done():
		}
		return summarizeRun(time.Now(), nil), nil
	}
	first, err := chat.startCopy(op)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := chat.startCopy(op); err == nil {
		t.Fatal("duplicate chat copy accepted")
	}
	if _, err := csv.startCopy(op); err != nil {
		t.Fatal("chat blocked CSV", err)
	}
	if _, err := csv.GetSyncJob(first.JobID); err == nil {
		t.Fatal("chat task leaked into CSV workflow")
	}
}

func TestCopyJobSurfacesFailureAndReleasesLock(t *testing.T) {
	jobs := NewJobService(nil)
	state := newJobState(nil)
	jobs.syncRunning = true
	jobs.runCopy(state, func(context.Context, ProgressListener) (ImportRunResult, error) {
		return ImportRunResult{}, errors.New("SMB unavailable")
	})
	result := state.view()
	if result.Status != "FAILED" || result.Message != "SMB unavailable" || jobs.syncRunning {
		t.Fatalf("result=%#v running=%v", result, jobs.syncRunning)
	}
}

func TestLocalArchiveDeletionRejectsActiveSync(t *testing.T) {
	jobs := NewJobService(nil)
	jobs.syncRunning = true
	called := false
	if err := jobs.WithCopyIdle(func() error { called = true; return nil }); err == nil || called {
		t.Fatal("deletion overlapped sync")
	}
	jobs.syncRunning = false
	if err := jobs.WithCopyIdle(func() error { called = true; return nil }); err != nil || !called {
		t.Fatal("idle management blocked", err)
	}
}
