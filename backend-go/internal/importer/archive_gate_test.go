package importer

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestArchiveGateSerializesSameSourceAndAllowsIndependentArchives(t *testing.T) {
	var gate archiveGate
	release, err := gate.acquire(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"sub", "group-chat"} {
		other, err := gate.acquire(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		other()
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		next, err := gate.acquire(ctx, "main")
		if next != nil {
			next()
		}
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled operation still waits on archive")
	}
	release()
	next, err := gate.acquire(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	next()
}

func TestJobCloseCancelsCopyAndRejectsNewTasks(t *testing.T) {
	jobs := NewJobService(nil)
	defer jobs.Close()
	started := make(chan struct{})
	first, err := jobs.startCopy(func(ctx context.Context, _ ProgressListener) (ImportRunResult, error) {
		close(started)
		<-ctx.Done()
		return ImportRunResult{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	jobs.Close()
	if _, err := jobs.startCopy(nil); err == nil {
		t.Fatal("closed job service accepted task")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state, err := jobs.GetSyncJob(first.JobID)
		if err != nil {
			t.Fatal(err)
		}
		if state.Status == "FAILED" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("closed copy did not reach failed terminal state")
}

func TestImportAndCopyJobHistoryHaveBoundedRetention(t *testing.T) {
	history := map[string]*jobState{}
	for index := 0; index < 20; index++ {
		state := newJobState(nil)
		state.startedAt = time.Unix(int64(index), 0)
		pruneJobHistory(history)
		history[state.jobID] = state
	}
	if len(history) != 8 {
		t.Fatalf("history contains %d jobs", len(history))
	}
	for _, state := range history {
		if state.startedAt.Before(time.Unix(12, 0)) {
			t.Fatal("old job retained")
		}
	}
}

func TestFlatArchiveRejectsDuplicateAndUnsafeRemoteNames(t *testing.T) {
	for _, files := range [][]RemoteLogFile{
		{{FileName: "2026-10-24.txt", Path: "one/2026-10-24.txt"}, {FileName: "2026-10-24.txt", Path: "two/2026-10-24.txt"}},
		{{FileName: "../2026-10-24.txt"}}, {{FileName: ".."}}, {{FileName: "bad:stream.txt"}},
	} {
		if err := validateArchiveNames(files); err == nil {
			t.Fatalf("accepted unsafe names: %#v", files)
		}
	}
	if err := validateArchiveNames([]RemoteLogFile{{FileName: "2026-10-23.txt"}, {FileName: "2026-10-24.txt"}}); err != nil {
		t.Fatal(err)
	}
}
