package importer

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestFailedRefreshRemovesFilesEvenWhenCreatorReturnsThem(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		cache := newLiveLogSnapshotCache(t.TempDir(), 120, time.UTC)
		file, err := os.CreateTemp(cache.root, liveLogSnapshotPrefix+"chat-*"+liveLogSnapshotSuffix)
		if err != nil {
			t.Fatal(err)
		}
		path := file.Name()
		file.Close()
		ctx, cancel := context.WithCancel(context.Background())
		flight := &liveLogSnapshotFlight{done: make(chan struct{}), ctx: ctx, cancel: cancel}
		cache.refresh("text", cache.day(cache.now()), flight, func(context.Context) ([]RemoteLogFile, error) {
			files := []RemoteLogFile{{Path: path}}
			if cancelled {
				cancel()
				return files, nil
			}
			return files, errors.New("partial download failed")
		})
		if flight.err == nil {
			t.Fatal("failed refresh accepted")
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed snapshot remains: %v", err)
		}
		if len(cache.entries) != 0 || len(cache.owned) != 0 {
			t.Fatal("failed snapshot published")
		}
		cache.close()
	}
}

func TestCacheCloseCancelsSharedDownload(t *testing.T) {
	cache := newLiveLogSnapshotCache(t.TempDir(), 120, time.UTC)
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := cache.acquire(context.Background(), "text", func(ctx context.Context) ([]RemoteLogFile, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		})
		done <- err
	}()
	<-started
	cache.close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("closed cache returned success")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shared download not cancelled")
	}
}

func TestCancelledQueryDoesNotCancelSharedRefreshForOtherQueries(t *testing.T) {
	cache := newLiveLogSnapshotCache(t.TempDir(), 120, time.UTC)
	defer cache.close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, allow := make(chan struct{}), make(chan struct{})
	create := func(ctx context.Context) ([]RemoteLogFile, error) {
		close(started)
		select {
		case <-allow:
			return []RemoteLogFile{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	first := make(chan error, 1)
	go func() {
		lease, err := cache.acquire(ctx, "text", create)
		if lease != nil {
			lease.Release()
		}
		first <- err
	}()
	<-started
	cancel()
	select {
	case err := <-first:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled waiter did not return")
	}
	close(allow)
	lease, err := cache.acquire(context.Background(), "text", create)
	if err != nil {
		t.Fatal(err)
	}
	lease.Release()
}
