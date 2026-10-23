// Package groupchat owns raw group-chat archives. It never imports chat content
// into statistics or dispatches CSV parsing jobs.
package groupchat

import (
	"context"
	"database/sql"
	"path"
	"path/filepath"
	"strings"

	"player-stats-backend-go/internal/auth"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/importer"
)

type CopyConfig struct {
	Enabled      bool   `json:"enabled"`
	SMBDirectory string `json:"smbDirectory"`
	FileGlob     string `json:"fileGlob"`
}

type CopyService struct {
	db          *sql.DB
	archiveRoot string
	copier      *importer.Service
	jobs        *importer.JobService
}

func NewCopyService(db *sql.DB, cfg config.Config, copier *importer.Service) *CopyService {
	return &CopyService{db: db, archiveRoot: filepath.Join(cfg.LogArchiveDirectory, "chat"), copier: copier, jobs: importer.NewJobService(copier)}
}

func (s *CopyService) Close() { s.jobs.Close() }

func NormalizeCopyConfig(c CopyConfig) (CopyConfig, error) {
	c.SMBDirectory = strings.Trim(strings.ReplaceAll(strings.TrimSpace(c.SMBDirectory), "\\", "/"), "/")
	for _, part := range strings.Split(c.SMBDirectory, "/") {
		if part == ".." {
			return c, auth.NewHTTPError(400, "聊天目录不能包含上级路径")
		}
	}
	if strings.ContainsAny(c.SMBDirectory, ":\x00") || len(c.SMBDirectory) > 1024 {
		return c, auth.NewHTTPError(400, "请填写 SMB 共享内的相对目录")
	}
	c.SMBDirectory = path.Clean(c.SMBDirectory)
	if c.SMBDirectory == "." {
		c.SMBDirectory = ""
	}
	if c.Enabled && c.SMBDirectory == "" {
		return c, auth.NewHTTPError(400, "请填写聊天日志目录")
	}
	c.FileGlob = strings.TrimSpace(c.FileGlob)
	if c.FileGlob == "" {
		c.FileGlob = "*.txt"
	}
	if strings.ContainsAny(c.FileGlob, "/\\\x00") || len(c.FileGlob) > 128 || !strings.HasSuffix(c.FileGlob, ".txt") {
		return c, auth.NewHTTPError(400, "聊天文件匹配必须是 TXT 文件名规则，例如 *.txt")
	}
	if _, err := filepath.Match(c.FileGlob, "2026-10-23.txt"); err != nil {
		return c, auth.NewHTTPError(400, "文件匹配规则无效")
	}
	return c, nil
}

func (s *CopyService) Config(ctx context.Context) (CopyConfig, error) {
	var c CopyConfig
	err := s.db.QueryRowContext(ctx, `select enabled, smb_directory, file_glob from group_chat_copy_config where id = 1`).Scan(&c.Enabled, &c.SMBDirectory, &c.FileGlob)
	return c, err
}

func (s *CopyService) SaveConfig(ctx context.Context, c CopyConfig) (CopyConfig, error) {
	normalized, err := NormalizeCopyConfig(c)
	if err != nil {
		return c, err
	}
	_, err = s.db.ExecContext(ctx, `update group_chat_copy_config set enabled = ?, smb_directory = ?, file_glob = ? where id = 1`, normalized.Enabled, normalized.SMBDirectory, normalized.FileGlob)
	return normalized, err
}

func (s *CopyService) source(ctx context.Context) (importer.ArchiveCopySource, error) {
	return s.archiveSource(ctx, true)
}
func (s *CopyService) archiveSource(ctx context.Context, requireEnabled bool) (importer.ArchiveCopySource, error) {
	c, err := s.Config(ctx)
	if err != nil {
		return importer.ArchiveCopySource{}, err
	}
	c, err = NormalizeCopyConfig(c)
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
