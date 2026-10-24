package settings

import (
	"context"
	"fmt"
	"player-stats-backend-go/internal/auth"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/storage"
)

type RuntimeResponse struct {
	Saved           config.RuntimeParameters `json:"saved"`
	Active          config.RuntimeParameters `json:"active"`
	RestartRequired bool                     `json:"restartRequired"`
}

// InitializeRuntime loads persisted tuning. Only the one-shot legacy migration
// tool reads retired environment defaults; server restarts never overwrite rows.
func InitializeRuntime(ctx context.Context, db *storage.DB, cfg config.Config) (config.Config, error) {
	p, err := readRuntime(ctx, db)
	if err != nil {
		return cfg, err
	}
	if err := p.Validate(); err != nil {
		return cfg, fmt.Errorf("已保存的运行参数无效：%w", err)
	}
	return cfg.WithRuntime(p), nil
}

func readRuntime(ctx context.Context, db *storage.DB) (config.RuntimeParameters, error) {
	var p config.RuntimeParameters
	err := db.QueryRowContext(ctx, `select live_cache_seconds, max_files_per_run, xray_retained_jobs from runtime_settings where id = 1`).Scan(&p.LiveCacheSeconds, &p.MaxFilesPerRun, &p.XrayRetainedJobs)
	return p, err
}

func (s *Service) Runtime(ctx context.Context) (RuntimeResponse, error) {
	p, err := readRuntime(ctx, s.db)
	active := s.cfg.RuntimeParameters()
	return RuntimeResponse{Saved: p, Active: active, RestartRequired: p != active}, err
}

func (s *Service) SaveRuntime(ctx context.Context, p config.RuntimeParameters) (RuntimeResponse, error) {
	if err := p.Validate(); err != nil {
		return RuntimeResponse{}, auth.NewHTTPError(400, err.Error())
	}
	_, err := s.db.ExecContext(ctx, `update runtime_settings set live_cache_seconds = ?, max_files_per_run = ?, xray_retained_jobs = ? where id = 1`, p.LiveCacheSeconds, p.MaxFilesPerRun, p.XrayRetainedJobs)
	if err != nil {
		return RuntimeResponse{}, err
	}
	return s.Runtime(ctx)
}
