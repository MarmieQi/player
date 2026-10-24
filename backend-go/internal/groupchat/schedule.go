package groupchat

import (
	"context"
	"fmt"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/importer"
	"player-stats-backend-go/internal/settings"
	"time"
)

type CopySchedule struct {
	Enabled   bool   `json:"enabled"`
	Time      string `json:"time"`
	SkipToday bool   `json:"skipToday"`
}

func (s *CopyService) Schedule(ctx context.Context) (CopySchedule, error) {
	var c CopySchedule
	// CSV and TXT automatic tasks share the existing global date policy.
	err := s.db.QueryRowContext(ctx, `select t.enabled,t.run_time,p.skip_today from task_schedules t join ingest_policy p on p.id=1 where t.source_id='group-chat' and t.kind='SYNC'`).Scan(&c.Enabled, &c.Time, &c.SkipToday)
	return c, err
}

func (s *CopyService) recordScheduled(ctx context.Context, job importer.ImportJobView, message string) error {
	if message == "" {
		message = fmt.Sprintf("自动同步 TXT：成功 %d，跳过 %d，失败 %d", job.ImportedFiles, job.SkippedFiles, job.FailedFiles)
	}
	return settings.NewService(s.db, config.Config{}).RecordTask(ctx, job.Status, message, "", job.ScannedFiles, job.ImportedFiles, job.SkippedFiles, job.FailedFiles)
}
func (s *CopyService) claimScheduled(ctx context.Context, now time.Time) (context.Context, bool, error) {
	id, ok, err := settings.NewService(s.db, config.Config{}).ClaimTask(ctx, "group-chat", "SYNC", now)
	return settings.WithRunID(ctx, id), ok, err
}
