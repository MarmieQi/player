package importer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/hirochachacha/go-smb2"

	"player-stats-backend-go/internal/config"
)

const (
	defaultLiveLogCacheTTL      = 120 * time.Second
	liveLogSnapshotReadTimeout  = 60 * time.Second
	liveLogSnapshotCleanupAge   = 30 * time.Minute
	liveLogSnapshotCleanupEvery = 10 * time.Minute
	liveLogSnapshotPrefix       = "player-log-query-"
	liveLogSnapshotPartSuffix   = ".part"
	liveLogSnapshotSuffix       = ".snapshot"
)

type liveLogSnapshotCache struct {
	root       string
	ttl        time.Duration
	location   *time.Location
	now        func() time.Time
	owned      map[string]*liveLogSnapshotEntry
	initErr    error
	mu         sync.Mutex
	entries    map[string]*liveLogSnapshotEntry
	refreshing map[string]*liveLogSnapshotFlight
	closed     bool
}

type liveLogSnapshotEntry struct {
	day       string
	files     []RemoteLogFile
	expiresAt time.Time
	refs      int
	retired   bool
}

type liveLogSnapshotFlight struct {
	done   chan struct{}
	err    error
	ctx    context.Context
	cancel context.CancelFunc
}

type liveLogSnapshotLease struct {
	cache *liveLogSnapshotCache
	entry *liveLogSnapshotEntry
	Files []RemoteLogFile
	once  sync.Once
}

func newLiveLogSnapshotCache(directory string, ttlSeconds int, location *time.Location) *liveLogSnapshotCache {
	if location == nil {
		location = time.Local
	}
	root, err := prepareLiveLogSnapshotRoot(directory)
	ttl := time.Duration(ttlSeconds) * time.Second
	if ttl < 10*time.Second || ttl > 10*time.Minute {
		ttl = defaultLiveLogCacheTTL
	}
	return &liveLogSnapshotCache{
		root:       root,
		location:   location,
		now:        time.Now,
		owned:      map[string]*liveLogSnapshotEntry{},
		ttl:        ttl,
		initErr:    err,
		entries:    map[string]*liveLogSnapshotEntry{},
		refreshing: map[string]*liveLogSnapshotFlight{},
	}
}

func prepareLiveLogSnapshotRoot(directory string) (string, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		directory = filepath.Join(os.TempDir(), "player-stats-live-log-query")
	}
	root, err := filepath.Abs(filepath.Clean(directory))
	if err != nil {
		return "", fmt.Errorf("解析当天日志临时目录失败: %w", err)
	}
	if filepath.Dir(root) == root {
		return "", errors.New("拒绝使用文件系统根目录作为当天日志临时目录")
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return "", fmt.Errorf("创建当天日志临时目录失败: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("解析当天日志临时目录失败: %w", err)
	}
	return filepath.Clean(resolved), nil
}

// RunLiveLogSnapshotCleanup removes expired cache entries with one shared ticker.
// Per-query leases keep snapshots alive while a scan is still using them.
func (s *Service) RunLiveLogSnapshotCleanup(ctx context.Context) {
	if s.liveSnapshots == nil {
		return
	}
	defer s.liveSnapshots.close()
	s.liveSnapshots.cleanup(time.Now())
	ticker := time.NewTicker(liveLogSnapshotCleanupEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.liveSnapshots.cleanup(now)
		}
	}
}

// CloseLiveLogSnapshots releases idle snapshots during graceful shutdown.
// Active scans keep their lease and delete the file when they finish.
func (s *Service) CloseLiveLogSnapshots() {
	if s.liveSnapshots != nil {
		s.liveSnapshots.close()
	}
}

func (c *liveLogSnapshotCache) close() {
	if c == nil || c.initErr != nil {
		return
	}
	remove := make([]RemoteLogFile, 0)
	c.mu.Lock()
	c.closed = true
	for _, flight := range c.refreshing {
		flight.cancel()
	}
	for key, entry := range c.entries {
		delete(c.entries, key)
		remove = append(remove, c.retireLocked(entry)...)
	}
	c.mu.Unlock()
	c.removeFiles(remove)
}

func (s *Service) acquireTodayLogSnapshot(ctx context.Context, source config.Source) (*liveLogSnapshotLease, error) {
	if s.liveSnapshots == nil {
		return nil, errors.New("当天日志临时缓存不可用")
	}
	return s.liveSnapshots.acquire(ctx, source.ID, func(refreshCtx context.Context) ([]RemoteLogFile, error) {
		return s.createTodayLogSnapshots(refreshCtx, source)
	})
}

func (c *liveLogSnapshotCache) acquire(
	ctx context.Context,
	key string,
	create func(context.Context) ([]RemoteLogFile, error),
) (*liveLogSnapshotLease, error) {
	if c.initErr != nil {
		return nil, c.initErr
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return nil, errors.New("当天日志临时缓存已关闭")
		}
		now := c.now()
		day := c.day(now)
		if entry := c.entries[key]; entry != nil && entry.day == day && now.Before(entry.expiresAt) && !entry.retired {
			entry.refs++
			lease := c.lease(entry)
			c.mu.Unlock()
			return lease, nil
		}
		flight := c.refreshing[key]
		if flight == nil {
			refreshCtx, cancel := context.WithTimeout(context.Background(), liveLogSnapshotReadTimeout)
			flight = &liveLogSnapshotFlight{done: make(chan struct{}), ctx: refreshCtx, cancel: cancel}
			c.refreshing[key] = flight
			// Refresh belongs to the shared cache, not the first querying task.
			go c.refresh(key, day, flight, create)
		}
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-flight.done:
			if flight.err != nil {
				return nil, flight.err
			}
		}
	}
}

func (c *liveLogSnapshotCache) refresh(key, day string, flight *liveLogSnapshotFlight, create func(context.Context) ([]RemoteLogFile, error)) {
	ctx := flight.ctx
	defer flight.cancel()
	files, err := create(ctx)
	if err == nil {
		err = ctx.Err()
	}
	var remove []RemoteLogFile
	c.mu.Lock()
	delete(c.refreshing, key)
	now := c.now()
	if c.closed {
		err = errors.New("当天日志临时缓存已关闭")
		remove = append(remove, files...)
	} else if day != c.day(now) {
		// Do not publish yesterday's download with a fresh TTL after midnight.
		// Wake waiters without an error so they retry for the new day.
		remove = append(remove, files...)
		err = nil
	} else if err == nil {
		if previous := c.entries[key]; previous != nil {
			remove = append(remove, c.retireLocked(previous)...)
		}
		entry := &liveLogSnapshotEntry{day: day, files: append([]RemoteLogFile(nil), files...), expiresAt: now.Add(c.ttl)}
		c.entries[key] = entry
		for _, file := range entry.files {
			c.owned[filepath.Clean(file.Path)] = entry
		}
	} else {
		// Even a failed or timed-out creator may have produced temporary files.
		remove = append(remove, files...)
	}
	flight.err = err
	close(flight.done)
	c.mu.Unlock()
	c.removeFiles(remove)
}

func (c *liveLogSnapshotCache) day(now time.Time) string {
	return now.In(c.location).Format("2006-01-02")
}

// Called under c.mu. Removing a cache key does not end ownership of leased files.
func (c *liveLogSnapshotCache) retireLocked(entry *liveLogSnapshotEntry) []RemoteLogFile {
	entry.retired = true
	if entry.refs != 0 {
		return nil
	}
	for _, file := range entry.files {
		path := filepath.Clean(file.Path)
		if c.owned[path] == entry {
			delete(c.owned, path)
		}
	}
	return entry.files
}

func (c *liveLogSnapshotCache) lease(entry *liveLogSnapshotEntry) *liveLogSnapshotLease {
	return &liveLogSnapshotLease{
		cache: c,
		entry: entry,
		Files: append([]RemoteLogFile(nil), entry.files...),
	}
}

func (l *liveLogSnapshotLease) Release() {
	if l == nil || l.cache == nil || l.entry == nil {
		return
	}
	l.once.Do(func() {
		var remove []RemoteLogFile
		l.cache.mu.Lock()
		if l.entry.refs > 0 {
			l.entry.refs--
		}
		if l.entry.retired && l.entry.refs == 0 {
			remove = append(remove, l.cache.retireLocked(l.entry)...)
		}
		l.cache.mu.Unlock()
		l.cache.removeFiles(remove)
	})
}

func (s *Service) createTodayLogSnapshots(ctx context.Context, source config.Source) ([]RemoteLogFile, error) {
	ctx, cancel := context.WithTimeout(ctx, liveLogSnapshotReadTimeout)
	defer cancel()

	settings, err := s.sourceSyncSettings(ctx, source)
	if err != nil {
		return nil, err
	}
	today := dateOnly(time.Now().In(s.cfg.Location), s.cfg.Location)
	created := make([]RemoteLogFile, 0, 1)
	err = s.withSMBShare(ctx, func(share *smb2.Share) error {
		files, err := listSMBFiles(share, settings.directory, settings.fileGlob, settings.recursive)
		if err != nil {
			return err
		}
		for _, file := range files {
			fileDate := extractLogDate(file.FileName, s.cfg.Location)
			if fileDate == nil || !fileDate.Equal(today) {
				continue
			}
			snapshot, err := snapshotRemoteLogFile(share, file, s.liveSnapshots.root, source.ID)
			if err != nil {
				return err
			}
			created = append(created, snapshot)
		}
		return nil
	})
	if err != nil {
		s.liveSnapshots.removeFiles(created)
		return nil, err
	}
	sort.Slice(created, func(i, j int) bool { return created[i].FileName < created[j].FileName })
	return created, nil
}

func snapshotRemoteLogFile(share *smb2.Share, remoteFile RemoteLogFile, root, serverID string) (RemoteLogFile, error) {
	if remoteFile.Size <= 0 {
		return RemoteLogFile{}, fmt.Errorf("当天 SMB 日志为空: %s", remoteFile.Path)
	}
	// go-smb2 Share.Open uses O_RDONLY and requests FILE_SHARE_WRITE, so the
	// server logger can keep appending while this bounded snapshot is copied.
	remote, err := share.Open(remoteFile.Path)
	if err != nil {
		return RemoteLogFile{}, fmt.Errorf("只读打开当天 SMB 日志失败 [%s]: %w", remoteFile.Path, err)
	}
	defer remote.Close()
	snapshotPath, snapshotSize, err := writeLiveLogSnapshot(remote, remoteFile.Size, root, serverID)
	if err != nil {
		return RemoteLogFile{}, fmt.Errorf("生成当天日志临时快照失败 [%s]: %w", remoteFile.Path, err)
	}
	return RemoteLogFile{
		Path:         snapshotPath,
		FileName:     remoteFile.FileName,
		Size:         snapshotSize,
		LastModified: remoteFile.LastModified,
		SourcePath:   remoteFile.Path,
	}, nil
}

// writeLiveLogSnapshot copies exactly the size observed during SMB listing.
// Bytes appended remotely after listing belong to the next snapshot. A final
// record without a newline is discarded because the writer may still own it.
func writeLiveLogSnapshot(reader io.Reader, size int64, root, serverID string) (string, int64, error) {
	return writeLiveSnapshot(reader, size, root, serverID, true)
}

func writeLiveSnapshot(reader io.Reader, size int64, root, serverID string, csv bool) (string, int64, error) {
	prefix := liveLogSnapshotPrefix + safeLiveLogName(serverID) + "-"
	temporary, err := os.CreateTemp(root, prefix+"*"+liveLogSnapshotPartSuffix)
	if err != nil {
		return "", 0, fmt.Errorf("创建当天日志临时文件失败: %w", err)
	}
	temporaryPath := temporary.Name()
	keep := false
	defer func() {
		_ = temporary.Close()
		if !keep {
			_ = removeLiveLogSnapshotFile(root, temporaryPath)
		}
	}()

	if _, err := io.CopyN(temporary, reader, size); err != nil {
		return "", 0, fmt.Errorf("读取固定长度日志内容失败: %w", err)
	}
	if err := trimSnapshotTail(temporary, !csv); err != nil {
		return "", 0, fmt.Errorf("整理当天日志临时快照失败: %w", err)
	}
	if csv {
		if err := validateLogSnapshotHeader(temporary); err != nil {
			return "", 0, fmt.Errorf("验证当天日志临时快照失败: %w", err)
		}
	}
	if err := temporary.Close(); err != nil {
		return "", 0, fmt.Errorf("关闭当天日志临时快照失败: %w", err)
	}
	snapshotPath := strings.TrimSuffix(temporaryPath, liveLogSnapshotPartSuffix) + liveLogSnapshotSuffix
	if err := os.Rename(temporaryPath, snapshotPath); err != nil {
		return "", 0, fmt.Errorf("发布当天日志临时快照失败: %w", err)
	}
	keep = true
	info, err := os.Stat(snapshotPath)
	if err != nil {
		_ = removeLiveLogSnapshotFile(root, snapshotPath)
		return "", 0, fmt.Errorf("读取当天日志临时快照信息失败: %w", err)
	}
	return snapshotPath, info.Size(), nil
}

func trimIncompleteLogTail(file *os.File) error {
	return trimSnapshotTail(file, false)
}

func trimSnapshotTail(file *os.File, allowEmpty bool) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		if allowEmpty {
			return nil
		}
		return errors.New("日志快照为空")
	}
	const chunkSize int64 = 4096
	buffer := make([]byte, chunkSize)
	for cursor := info.Size(); cursor > 0; {
		count := min(chunkSize, cursor)
		cursor -= count
		read, err := file.ReadAt(buffer[:count], cursor)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		for index := read - 1; index >= 0; index-- {
			if buffer[index] == '\n' {
				return file.Truncate(cursor + int64(index) + 1)
			}
		}
	}
	if allowEmpty {
		return file.Truncate(0)
	}
	return errors.New("日志快照中没有完整记录")
}

func validateLogSnapshotHeader(file *os.File) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	line, err := bufio.NewReaderSize(file, 64*1024).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if !isHeader(splitPrefix(strings.TrimRight(line, "\r\n"))) {
		return errors.New("日志表头格式无效")
	}
	return nil
}

func safeLiveLogName(value string) string {
	value = strings.Map(func(character rune) rune {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' || character == '_' {
			return character
		}
		return '-'
	}, strings.TrimSpace(value))
	if value == "" {
		return "server"
	}
	return value
}

func (c *liveLogSnapshotCache) cleanup(now time.Time) {
	if c == nil || c.initErr != nil {
		return
	}
	remove := make([]RemoteLogFile, 0)
	day := c.day(now)
	c.mu.Lock()
	for key, entry := range c.entries {
		if entry.day != day || !now.Before(entry.expiresAt) {
			delete(c.entries, key)
			remove = append(remove, c.retireLocked(entry)...)
		}
	}
	c.mu.Unlock()
	c.removeFiles(remove)
	c.cleanupOrphans(now)
}

func (c *liveLogSnapshotCache) cleanupOrphans(now time.Time) {
	entries, err := os.ReadDir(c.root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type()&os.ModeSymlink != 0 || !isLiveLogSnapshotName(name) {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || now.Sub(info.ModTime()) < liveLogSnapshotCleanupAge {
			continue
		}
		candidate := filepath.Join(c.root, name)
		// Recheck current ownership, rather than a stale copy of the cache keys.
		c.mu.Lock()
		_, protected := c.owned[filepath.Clean(candidate)]
		c.mu.Unlock()
		if protected {
			continue
		}
		_ = removeLiveLogSnapshotFile(c.root, candidate)
	}
}

func (c *liveLogSnapshotCache) removeFiles(files []RemoteLogFile) {
	for _, file := range files {
		_ = removeLiveLogSnapshotFile(c.root, file.Path)
	}
}

func removeLiveLogSnapshotFile(root, candidate string) error {
	// Cleanup is intentionally non-recursive and only accepts owned, direct-child
	// regular files from the dedicated snapshot directory.
	if !isDirectChild(root, candidate) || !isLiveLogSnapshotName(filepath.Base(candidate)) {
		return errors.New("拒绝清理无效的当天日志临时文件")
	}
	info, err := os.Lstat(candidate)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("拒绝清理非普通当天日志临时文件")
	}
	return os.Remove(candidate)
}

func isLiveLogSnapshotName(name string) bool {
	return strings.HasPrefix(name, liveLogSnapshotPrefix) &&
		(strings.HasSuffix(name, liveLogSnapshotPartSuffix) || strings.HasSuffix(name, liveLogSnapshotSuffix))
}

func mergeTodayLogSnapshotFiles(localFiles, liveFiles []RemoteLogFile, location *time.Location) []RemoteLogFile {
	today := dateOnly(time.Now().In(location), location)
	current := make([]RemoteLogFile, 0, len(liveFiles))
	for _, file := range liveFiles {
		if day := extractLogDate(file.FileName, location); day != nil && day.Equal(today) {
			current = append(current, file)
		}
	}
	// A query can cross midnight after acquiring its lease. Never let an old
	// snapshot hide today's local file or duplicate yesterday's archived logs.
	if len(current) == 0 {
		return localFiles
	}
	liveFiles = current
	merged := make([]RemoteLogFile, 0, len(localFiles)+len(liveFiles))
	for _, file := range localFiles {
		fileDate := extractLogDate(file.FileName, location)
		// A successful live snapshot is newer than any manually archived copy of
		// today's file, so scan only one source for that date.
		if fileDate != nil && fileDate.Equal(today) {
			continue
		}
		merged = append(merged, file)
	}
	merged = append(merged, liveFiles...)
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].FileName != merged[j].FileName {
			return merged[i].FileName < merged[j].FileName
		}
		return merged[i].Path < merged[j].Path
	})
	return merged
}
