package importer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"player-stats-backend-go/internal/config"
	"testing"
	"time"
)

func TestLogQueryCancellationIsJobScopedAndCompletesOnExit(t *testing.T) {
	state := newLogQueryState("main", "主服", logQueryCriteria{queryType: queryTypeCoordinate})
	key := logQueryStateKey("main", queryTypeCoordinate)
	service := &LogQueryService{importer: &Service{}, latest: map[string]*logQueryState{key: state}, running: map[string]bool{key: true}}
	if _, err := service.Cancel(context.Background(), "main", queryTypeCoordinate, "old-job"); err == nil {
		t.Fatal("stale page cancelled a new job")
	}
	if state.task.ctx.Err() != nil {
		t.Fatal("stale request signalled cancellation")
	}
	result, err := service.Cancel(context.Background(), "main", queryTypeCoordinate, state.jobID)
	if err != nil || result.Status != "CANCELLING" {
		t.Fatalf("cancel result: %v %v", result.Status, err)
	}
	if _, err := service.Clear(context.Background(), "main", queryTypeCoordinate); err == nil {
		t.Fatal("cleared before worker exited")
	}
	service.run(state, configlessSource{}, key)
	result = state.view(1, 100)
	if result.Status != "CANCELLED" || result.FinishedAt == nil || service.running[key] {
		t.Fatalf("worker did not finish cancellation: %#v", result)
	}
	result, err = service.Cancel(context.Background(), "main", queryTypeCoordinate, state.jobID)
	if err != nil || result.Status != "CANCELLED" {
		t.Fatal("repeat cancel changed terminal state")
	}
}

func TestFinishedQueryCannotBeChangedToCancelled(t *testing.T) {
	state := newLogQueryState("main", "主服", logQueryCriteria{})
	state.markFinished()
	state.mu.Lock()
	err := requestQueryCancellation(state.task, state.jobID, state.jobID, &state.status, &state.message)
	state.mu.Unlock()
	if err != nil || state.view(1, 100).Status != "FINISHED" || state.task.ctx.Err() != nil {
		t.Fatal("finished task was cancelled")
	}
	state.task.cancel()
}

func TestLogScanStopsOnCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.csv")
	if err := os.WriteFile(path, []byte("2026-10-23,00:00:00,Alex,破坏方块,0,0,0,主世界,0,0,0,主世界,minecraft:stone,-\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newLogQueryState("main", "主服", logQueryCriteria{queryType: queryTypePlayerKeyword})
	state.task.cancel()
	err := (&LogQueryService{}).scanFile(state, RemoteLogFile{Path: path})
	if !errors.Is(err, context.Canceled) || state.scannedRows != 0 {
		t.Fatalf("scan continued after cancellation: %v", err)
	}
}

func TestXrayCancellationAndDiskCleanup(t *testing.T) {
	state := newXrayState(config.Source{ID: "main"}, xrayCriteria{serverID: "main"})
	service := &XrayAnalysisService{latest: map[string]*xrayState{"main": state}, running: map[string]bool{"main": true}}
	result, err := service.Cancel(context.Background(), "main", state.jobID)
	if err != nil || result.Status != "CANCELLING" {
		t.Fatalf("xray cancel: %s %v", result.Status, err)
	}
	state.markRunning()
	if state.view().Status != "CANCELLING" {
		t.Fatal("starting worker overwrote cancellation")
	}
	root := t.TempDir()
	store, err := newXrayDiskStore(root, state.jobID, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.resultsContext(state.task.ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("analysis did not stop: %v", err)
	}
	if err := store.cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.jobDirectory); !os.IsNotExist(err) {
		t.Fatal("cancelled task temporary directory remained")
	}
	events := make([]xrayEvent, 100)
	if result := analyzePlayerContext(state.task.ctx, "Alex", events); result.PlayerName != "" {
		t.Fatal("cancelled analysis produced a conclusion")
	}
}

func TestCancelledSnapshotWaiterDoesNotCancelSharedRefresh(t *testing.T) {
	cache := newLiveLogSnapshotCache(t.TempDir(), 120, time.UTC)
	defer cache.close()
	started, finish := make(chan struct{}), make(chan struct{})
	refreshContext := make(chan context.Context, 1)
	create := func(ctx context.Context) ([]RemoteLogFile, error) {
		close(started)
		refreshContext <- ctx
		<-finish
		return []RemoteLogFile{}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		lease, err := cache.acquire(ctx, "main", create)
		if lease != nil {
			lease.Release()
		}
		done <- err
	}()
	<-started
	shared := <-refreshContext
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled waiter did not exit")
	}
	if shared.Err() != nil {
		t.Fatal("one task cancelled the shared refresh")
	}
	close(finish)
	lease, err := cache.acquire(context.Background(), "main", func(context.Context) ([]RemoteLogFile, error) { t.Fatal("download was repeated"); return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	lease.Release()
}
