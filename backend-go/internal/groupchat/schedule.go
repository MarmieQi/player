package groupchat

import (
	"context"
	"fmt"
	"player-stats-backend-go/internal/auth"
	"player-stats-backend-go/internal/importer"
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
	err := s.db.QueryRowContext(ctx, `select chat.auto_enabled, chat.auto_time, sync.skip_today
 from group_chat_copy_config chat join sync_config sync on sync.id = 1 where chat.id = 1`).Scan(&c.Enabled, &c.Time, &c.SkipToday)
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
	_, err = s.db.ExecContext(ctx, `update group_chat_copy_config set auto_enabled = ?, auto_time = ? where id = 1`, c.Enabled, c.Time)
	if err != nil {
		return c, err
	}
	return s.Schedule(ctx)
}

func (s *CopyService) recordScheduled(ctx context.Context, job importer.ImportJobView, message string) error {
	if message == "" {
		message = fmt.Sprintf("自动同步 TXT：成功 %d，跳过 %d，失败 %d", job.ImportedFiles, job.SkippedFiles, job.FailedFiles)
	}
	_, err := s.db.ExecContext(ctx, `insert into auto_task_logs
 (created_at,server_id,server_name,task_type,task_label,status,message,scanned_files,success_files,skipped_files,failed_files)
 values (?, 'group-chat', '群服聊天', 'CHAT_SYNC', '群服聊天：自动同步 TXT', ?, ?, ?, ?, ?, ?)`, time.Now().UTC(), job.Status, message, job.ScannedFiles, job.ImportedFiles, job.SkippedFiles, job.FailedFiles)
	return err
}
