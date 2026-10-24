// Package groupchat owns raw group-chat archives. It never imports chat content
// into statistics or dispatches CSV parsing jobs.
package groupchat

import (
	"context"
	"path/filepath"
	"player-stats-backend-go/internal/storage"

	"player-stats-backend-go/internal/auth"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/importer"
	"player-stats-backend-go/internal/settings"
)

type CopyService struct {
	db          *storage.DB
	archiveRoot string
	copier      *importer.Service
	jobs        *importer.JobService
}

func NewCopyService(db *storage.DB, cfg config.Config, copier *importer.Service) *CopyService {
	return &CopyService{db: db, archiveRoot: filepath.Join(cfg.LogArchiveDirectory, "chat"), copier: copier, jobs: importer.NewJobService(copier)}
}

func (s *CopyService) Close() { s.jobs.Close() }

func (s *CopyService) Config(ctx context.Context) (settings.ChatDirectory, error) {
	var c settings.ChatDirectory
	err := s.db.QueryRowContext(ctx, `select enabled,smb_directory,smb_file_glob from log_sources where source_id='group-chat'`).Scan(&c.Enabled, &c.SMBDirectory, &c.FileGlob)
	return c, err
}

func (s *CopyService) source(ctx context.Context) (importer.ArchiveCopySource, error) {
	return s.archiveSource(ctx, true)
}
func (s *CopyService) archiveSource(ctx context.Context, requireEnabled bool) (importer.ArchiveCopySource, error) {
	c, err := s.Config(ctx)
	if err != nil {
		return importer.ArchiveCopySource{}, err
	}
	c, err = settings.NormalizeChatDirectory(c)
	if err != nil {
		return importer.ArchiveCopySource{}, err
	}
	if requireEnabled && !c.Enabled {
		return importer.ArchiveCopySource{}, auth.NewHTTPError(400, "请先启用并保存群服聊天同步配置")
	}
	return importer.ArchiveCopySource{Local: config.Source{ID: "group-chat", Name: "群服聊天", Directory: s.archiveRoot, FileGlob: c.FileGlob, Enabled: true}, RemoteDirectory: c.SMBDirectory, FileGlob: c.FileGlob}, nil
}

func (s *CopyService) Files(ctx context.Context, skipToday bool) ([]importer.ImportFileStatus, error) {
	source, err := s.source(ctx)
	if err != nil {
		return nil, err
	}
	return s.copier.ListSMBArchive(ctx, source, skipToday)
}

func (s *CopyService) Start(ctx context.Context, skipToday bool) (importer.ImportJobView, error) {
	source, err := s.source(ctx)
	if err != nil {
		return importer.ImportJobView{}, err
	}
	return s.jobs.StartArchiveCopy(source, skipToday)
}

func (s *CopyService) Job(id string) (importer.ImportJobView, error) { return s.jobs.GetSyncJob(id) }

func (s *CopyService) DirectoryFiles(ctx context.Context) ([]importer.RemoteLogFile, error) {
	source, err := s.archiveSource(ctx, false)
	if err != nil {
		return nil, err
	}
	if source.RemoteDirectory == "" {
		return nil, auth.NewHTTPError(400, "请先保存聊天目录配置")
	}
	return s.copier.ListSMBDirectory(ctx, source.RemoteDirectory, source.FileGlob)
}

func (s *CopyService) LocalFiles(ctx context.Context) ([]importer.ImportFileStatus, error) {
	source, err := s.archiveSource(ctx, false)
	if err != nil {
		return nil, err
	}
	return s.copier.ListLocalSMBArchive(source)
}
func (s *CopyService) DeleteLocalFiles(ctx context.Context, request importer.DeleteImportRecordsRequest) ([]importer.DeleteImportRecordResult, error) {
	if len(request.Files) == 0 || len(request.Files) > 1000 {
		return nil, auth.NewHTTPError(400, "请选择 1 到 1000 个本地 TXT 文件")
	}
	source, err := s.archiveSource(ctx, false)
	if err != nil {
		return nil, err
	}
	results := make([]importer.DeleteImportRecordResult, 0, len(request.Files))
	err = s.jobs.WithCopyIdle(func() error {
		for _, file := range request.Files {
			if err := ctx.Err(); err != nil {
				return err
			}
			result, err := s.copier.DeleteSMBArchiveFile(source, file.RemotePath)
			if err != nil {
				result = importer.DeleteImportRecordResult{ServerID: source.Local.ID, RemotePath: file.RemotePath, Deleted: false, Message: err.Error()}
			}
			results = append(results, result)
		}
		return nil
	})
	return results, err
}
