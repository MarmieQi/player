package importer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testLogHeader = "日期,时间,玩家名称,行为,X坐标,Y坐标,Z坐标,维度,X坐标2,Y坐标2,Z坐标2,维度2,详细信息1,详细信息2\r\n"

func TestWriteLiveLogSnapshotCopiesFixedPrefixAndDropsIncompleteTail(t *testing.T) {
	root := t.TempDir()
	complete := testLogHeader + "2026-08-26,14:35:20,Steve,放置方块,1,2,3,overworld,4,5,6,overworld,stone,-\r\n"
	visibleAtStart := complete + "2026-08-26,14:35:21,Alex,使用桶放置,1,2"
	remoteAfterAppend := visibleAtStart + ",3,overworld,-,-,-,-,water,-\r\n"

	path, size, err := writeLiveLogSnapshot(strings.NewReader(remoteAfterAppend), int64(len(visibleAtStart)), root, "main")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = removeLiveLogSnapshotFile(root, path) })

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != complete {
		t.Fatalf("snapshot content = %q, want fixed complete prefix %q", content, complete)
	}
	if size != int64(len(complete)) {
		t.Fatalf("snapshot size = %d, want %d", size, len(complete))
	}
}

func TestLiveLogSnapshotCacheReusesAndRetiresFilesSafely(t *testing.T) {
	cache := newLiveLogSnapshotCache(t.TempDir(), 120, time.UTC)
	created := 0
	create := func(context.Context) ([]RemoteLogFile, error) {
		created++
		file, err := os.CreateTemp(cache.root, liveLogSnapshotPrefix+"main-*"+liveLogSnapshotSuffix)
		if err != nil {
			return nil, err
		}
		path := file.Name()
		if err := file.Close(); err != nil {
			return nil, err
		}
		return []RemoteLogFile{{Path: path, FileName: "player_actions_2026-08-26.csv"}}, nil
	}

	first, err := cache.acquire(context.Background(), "main", create)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.acquire(context.Background(), "main", create)
	if err != nil {
		t.Fatal(err)
	}
	if created != 1 || first.Files[0].Path != second.Files[0].Path {
		t.Fatalf("fresh cache was not reused: created=%d first=%q second=%q", created, first.Files[0].Path, second.Files[0].Path)
	}

	cache.mu.Lock()
	cache.entries["main"].expiresAt = time.Now().Add(-time.Second)
	cache.mu.Unlock()
	third, err := cache.acquire(context.Background(), "main", create)
	if err != nil {
		t.Fatal(err)
	}
	if created != 2 || third.Files[0].Path == first.Files[0].Path {
		t.Fatalf("expired cache was not refreshed: created=%d", created)
	}
	if _, err := os.Stat(first.Files[0].Path); err != nil {
		t.Fatalf("active retired snapshot was removed early: %v", err)
	}

	first.Release()
	if _, err := os.Stat(first.Files[0].Path); err != nil {
		t.Fatalf("snapshot with another active lease was removed early: %v", err)
	}
	second.Release()
	if _, err := os.Stat(first.Files[0].Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retired snapshot still exists after final release: %v", err)
	}
	third.Release()
	cache.cleanup(time.Now().Add(3 * time.Minute))
}

func TestLiveLogSnapshotCacheCoalescesConcurrentRefreshes(t *testing.T) {
	cache := newLiveLogSnapshotCache(t.TempDir(), 120, time.UTC)
	var created atomic.Int32
	started := make(chan struct{})
	allowCreate := make(chan struct{})
	create := func(context.Context) ([]RemoteLogFile, error) {
		if created.Add(1) == 1 {
			close(started)
		}
		<-allowCreate
		return []RemoteLogFile{}, nil
	}

	const callers = 8
	var wait sync.WaitGroup
	errorsByCaller := make(chan error, callers)
	wait.Add(callers)
	for range callers {
		go func() {
			defer wait.Done()
			lease, err := cache.acquire(context.Background(), "main", create)
			if err == nil {
				lease.Release()
			}
			errorsByCaller <- err
		}()
	}
	<-started
	close(allowCreate)
	wait.Wait()
	close(errorsByCaller)
	for err := range errorsByCaller {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count := created.Load(); count != 1 {
		t.Fatalf("concurrent refresh count = %d, want 1", count)
	}
	cache.close()
}

func TestLiveLogSnapshotCacheCloseKeepsActiveLeaseUntilRelease(t *testing.T) {
	cache := newLiveLogSnapshotCache(t.TempDir(), 120, time.UTC)
	file, err := os.CreateTemp(cache.root, liveLogSnapshotPrefix+"main-*"+liveLogSnapshotSuffix)
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	lease, err := cache.acquire(context.Background(), "main", func(context.Context) ([]RemoteLogFile, error) {
		return []RemoteLogFile{{Path: path, FileName: "player_actions_2026-08-26.csv"}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	cache.close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("active snapshot was removed during close: %v", err)
	}
	if _, err := cache.acquire(context.Background(), "main", func(context.Context) ([]RemoteLogFile, error) {
		return nil, nil
	}); err == nil {
		t.Fatal("closed cache accepted a new acquire")
	}
	lease.Release()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot still exists after releasing closed cache lease: %v", err)
	}
}

func TestLiveLogSnapshotCleanupOnlyDeletesOwnedStaleFiles(t *testing.T) {
	cache := newLiveLogSnapshotCache(t.TempDir(), 120, time.UTC)
	owned := filepath.Join(cache.root, liveLogSnapshotPrefix+"main-old"+liveLogSnapshotSuffix)
	unrelated := filepath.Join(cache.root, "do-not-delete.txt")
	ownedDirectory := filepath.Join(cache.root, liveLogSnapshotPrefix+"directory"+liveLogSnapshotSuffix)
	if err := os.WriteFile(owned, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unrelated, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(ownedDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(owned, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(unrelated, old, old); err != nil {
		t.Fatal(err)
	}

	cache.cleanup(time.Now())
	if _, err := os.Stat(owned); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned stale snapshot was not removed: %v", err)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatalf("unrelated file was removed: %v", err)
	}
	if info, err := os.Stat(ownedDirectory); err != nil || !info.IsDir() {
		t.Fatalf("directory was removed or changed: %v", err)
	}
	outside := filepath.Join(t.TempDir(), liveLogSnapshotPrefix+"outside"+liveLogSnapshotSuffix)
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeLiveLogSnapshotFile(cache.root, outside); err == nil {
		t.Fatal("cleanup accepted a file outside its dedicated root")
	}
}

func TestMergeTodayLogSnapshotFilesReplacesArchivedTodayOnly(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	today := time.Now().In(location).Format("2006-01-02")
	yesterday := time.Now().In(location).AddDate(0, 0, -1).Format("2006-01-02")
	local := []RemoteLogFile{
		{Path: "history", FileName: "player_actions_" + yesterday + ".csv"},
		{Path: "archived-today", FileName: "player_actions_" + today + ".csv"},
	}
	live := []RemoteLogFile{{Path: "live-today", FileName: "player_actions_" + today + ".csv"}}

	merged := mergeTodayLogSnapshotFiles(local, live, location)
	if len(merged) != 2 {
		t.Fatalf("merged files = %d, want 2", len(merged))
	}
	paths := map[string]bool{merged[0].Path: true, merged[1].Path: true}
	if !paths["history"] || !paths["live-today"] || paths["archived-today"] {
		t.Fatalf("unexpected merged paths: %#v", paths)
	}
}

func TestLiveLogSnapshotRollsOverByConfiguredTimezone(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	cache := newLiveLogSnapshotCache(t.TempDir(), 120, location)
	defer cache.close()
	var clock atomic.Int64
	before := time.Date(2026, 10, 23, 23, 59, 50, 0, location)
	clock.Store(before.UnixNano())
	cache.now = func() time.Time { return time.Unix(0, clock.Load()) }
	var created atomic.Int32
	create := func(context.Context) ([]RemoteLogFile, error) {
		created.Add(1)
		return []RemoteLogFile{{FileName: "player_actions_" + cache.day(cache.now()) + ".csv"}}, nil
	}
	first, err := cache.acquire(context.Background(), "main", create)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	same, err := cache.acquire(context.Background(), "main", create)
	if err != nil {
		t.Fatal(err)
	}
	same.Release()
	if created.Load() != 1 {
		t.Fatal("same-day snapshot was not reused")
	}
	// UTC remains October 23, but the configured server timezone is now October 24.
	clock.Store(before.Add(20 * time.Second).UnixNano())
	second, err := cache.acquire(context.Background(), "main", create)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release()
	if created.Load() != 2 || second.Files[0].FileName != "player_actions_2026-10-24.csv" {
		t.Fatalf("previous day reused: %v", second.Files)
	}
	if !first.entry.retired {
		t.Fatal("previous day's active snapshot was not retired")
	}
}

func TestLiveLogSnapshotDiscardsDownloadThatCrossesMidnight(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	cache := newLiveLogSnapshotCache(t.TempDir(), 120, location)
	defer cache.close()
	var clock atomic.Int64
	before := time.Date(2026, 10, 23, 23, 59, 50, 0, location)
	clock.Store(before.UnixNano())
	cache.now = func() time.Time { return time.Unix(0, clock.Load()) }
	var calls atomic.Int32
	var oldPath string
	create := func(context.Context) ([]RemoteLogFile, error) {
		day := cache.day(cache.now())
		file, err := os.CreateTemp(cache.root, liveLogSnapshotPrefix+"midnight-*"+liveLogSnapshotSuffix)
		if err != nil {
			return nil, err
		}
		file.Close()
		if calls.Add(1) == 1 {
			oldPath = file.Name()
			clock.Store(before.Add(20 * time.Second).UnixNano())
		}
		return []RemoteLogFile{{Path: file.Name(), FileName: "player_actions_" + day + ".csv"}}, nil
	}
	lease, err := cache.acquire(context.Background(), "main", create)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	if calls.Load() != 2 || lease.Files[0].FileName != "player_actions_2026-10-24.csv" {
		t.Fatalf("midnight download was published: %v", lease.Files)
	}
	// File deletion runs outside the cache mutex; ownership must already be
	// released when the new-day snapshot becomes available.
	cache.mu.Lock()
	_, owned := cache.owned[oldPath]
	cache.mu.Unlock()
	if owned {
		t.Fatal("discarded midnight snapshot retained ownership")
	}
}

func TestRetiredSnapshotIsProtectedUntilLastLeaseIsReleased(t *testing.T) {
	cache := newLiveLogSnapshotCache(t.TempDir(), 120, time.UTC)
	defer cache.close()
	create := func(context.Context) ([]RemoteLogFile, error) {
		file, err := os.CreateTemp(cache.root, liveLogSnapshotPrefix+"active-*"+liveLogSnapshotSuffix)
		if err != nil {
			return nil, err
		}
		file.Close()
		return []RemoteLogFile{{Path: file.Name()}}, nil
	}
	old, err := cache.acquire(context.Background(), "main", create)
	if err != nil {
		t.Fatal(err)
	}
	oldAgain, err := cache.acquire(context.Background(), "main", create)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Release()
	defer oldAgain.Release()
	stale := time.Now().Add(-31 * time.Minute)
	if err := os.Chtimes(old.Files[0].Path, stale, stale); err != nil {
		t.Fatal(err)
	}
	cache.mu.Lock()
	cache.entries["main"].expiresAt = time.Now().Add(-time.Second)
	cache.mu.Unlock()
	fresh, err := cache.acquire(context.Background(), "main", create)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Release()
	cache.cleanup(time.Now())
	if _, err := os.Stat(old.Files[0].Path); err != nil {
		t.Fatalf("retired active file was removed: %v", err)
	}
	old.Release()
	cache.cleanup(time.Now())
	if _, err := os.Stat(oldAgain.Files[0].Path); err != nil {
		t.Fatalf("remaining lease lost its file: %v", err)
	}
	oldAgain.Release()
	if _, err := os.Stat(old.Files[0].Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file remained after last lease: %v", err)
	}
	cache.mu.Lock()
	_, owned := cache.owned[old.Files[0].Path]
	cache.mu.Unlock()
	if owned {
		t.Fatal("released retired file remained in ownership registry")
	}
}

func TestMergeSnapshotFromPreviousDayDoesNotHideTodayOrDuplicateYesterday(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	today := time.Now().In(location)
	yesterday := today.AddDate(0, 0, -1).Format("2006-01-02")
	local := []RemoteLogFile{{Path: "archive-yesterday", FileName: "player_actions_" + yesterday + ".csv"}, {Path: "archive-today", FileName: "player_actions_" + today.Format("2006-01-02") + ".csv"}}
	live := []RemoteLogFile{{Path: "stale-snapshot", FileName: "player_actions_" + yesterday + ".csv"}}
	merged := mergeTodayLogSnapshotFiles(local, live, location)
	if len(merged) != 2 || merged[0].Path != "archive-yesterday" || merged[1].Path != "archive-today" {
		t.Fatalf("stale snapshot corrupted merged files: %v", merged)
	}
}
