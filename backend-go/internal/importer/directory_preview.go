package importer

import (
	"context"
	"time"

	"github.com/hirochachacha/go-smb2"
	"player-stats-backend-go/internal/auth"
)

// ListSMBDirectory is a configuration preview: remote metadata only, without
// creating archives, checking local copies, reading import state or date policy.
func (s *Service) ListSMBDirectory(ctx context.Context, directory, glob string) ([]RemoteLogFile, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	result := []RemoteLogFile{}
	err := s.withSMBShare(ctx, func(share *smb2.Share) error {
		files, err := listSMBFiles(share, directory, glob, false)
		if err != nil {
			return err
		}
		result = limitFiles(files, s.cfg.MaxFilesPerRun)
		return nil
	})
	return result, err
}

func (s *Service) ListSourceDirectoryFiles(ctx context.Context, id string) ([]RemoteLogFile, error) {
	// Disabled sources can still be inspected while configuring them.
	for _, source := range s.cfg.Sources() {
		if source.ID != id {
			continue
		}
		config, err := s.sourceSyncSettings(ctx, source)
		if err != nil {
			return nil, err
		}
		return s.ListSMBDirectory(ctx, config.directory, config.fileGlob)
	}
	return nil, auth.NewHTTPError(404, "日志源不存在")
}
