package groupchat

import (
	"context"
	"fmt"
	"player-stats-backend-go/internal/auth"
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

func normalizeSchedule(c CopySchedule) (CopySchedule, error) {
	if len(c.Time) != 5 {
		return c, auth.NewHTTPError(400, "同步时间必须是 HH:mm 格式")
	}
	if _, err := time.Parse("15:04", c.Time); err != nil {
		return c, auth.NewHTTPError(400, "同步时间必须是有效的 HH:mm 时间")
	}
	return c, nil
}

func (s *CopyService) Schedule(ctx context.Context) (CopySchedule, error) {
	var c CopySchedule
	// CSV and TXT automatic tasks share the existing global date policy.
	err := s.db.QueryRowContext(ctx, `select t.enabled,t.run_time,p.skip_today from task_schedules t join ingest_policy p on p.id=1 where t.source_id='group-chat' and t.kind='SYNC'`).Scan(&c.Enabled, &c.Time, &c.SkipToday)
	return c, err
}

func (s *CopyService) SaveSchedule(ctx context.Context, c CopySchedule) (CopySchedule, error) {
	c, err := normalizeSchedule(c)
	if err != nil {
		return c, err
	}
	if c.Enabled {
		if _, err := s.source(ctx); err != nil {
			return c, err
		}
	}
	_, err = s.db.ExecContext(ctx, `update task_schedules set enabled=?,run_time=? where source_id='group-chat' and kind='SYNC'`, c.Enabled, c.Time)
	if err != nil {
		return c, err
	}
	return s.Schedule(ctx)
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
