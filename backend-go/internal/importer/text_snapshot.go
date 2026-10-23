package importer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"
)

// ArchiveSnapshot is a lease on immutable files owned by the shared live cache.
// Release must be called after scanning; callers never delete these files.
type ArchiveSnapshot struct {
	Files   []RemoteLogFile
	Release func()
}

// AcquireTodayTextSnapshot shares CSV's TTL, refresh flights, leases and cleanup.
// The key includes the remote configuration so changing directories cannot reuse
// a snapshot from the previous source. It ignores batch-sync skipToday policy.
func (s *Service) AcquireTodayTextSnapshot(ctx context.Context, archive ArchiveCopySource) (ArchiveSnapshot, error) {
	if s.liveSnapshots == nil {
		return ArchiveSnapshot{}, fmt.Errorf("当天聊天缓存不可用")
	}
	key := strings.Join([]string{"text", archive.Local.ID, archive.RemoteDirectory, archive.FileGlob}, "\x00")
	lease, err := s.liveSnapshots.acquire(ctx, key, func(ctx context.Context) ([]RemoteLogFile, error) {
		today := dateOnly(time.Now().In(s.cfg.Location), s.cfg.Location)
		created := make([]RemoteLogFile, 0)
		var total int64
		err := s.withSMBShare(ctx, func(share *smb2.Share) error {
			files, err := listSMBFiles(share, archive.RemoteDirectory, archive.FileGlob, false)
			if err != nil {
				return err
			}
			for _, file := range files {
				day := extractLogDate(file.FileName, s.cfg.Location)
				if day == nil || !day.Equal(today) {
					continue
				}
				total += file.Size
				if file.Size < 0 || total > 128<<20 {
					return fmt.Errorf("当天聊天文件超过 128 MB，无法生成查询快照")
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				remote, err := share.Open(file.Path)
				if err != nil {
					return err
				}
				name, size, copyErr := writeLiveSnapshot(remote, file.Size, s.liveSnapshots.root, "chat", false)
				_ = remote.Close()
				if copyErr != nil {
					return copyErr
				}
				file.SourcePath, file.Path, file.Size = file.Path, name, size
				created = append(created, file)
			}
			return nil
		})
		if err != nil {
			s.liveSnapshots.removeFiles(created)
			return nil, err
		}
		return created, nil
	})
	if err != nil {
		return ArchiveSnapshot{}, err
	}
	return ArchiveSnapshot{Files: lease.Files, Release: lease.Release}, nil
}
