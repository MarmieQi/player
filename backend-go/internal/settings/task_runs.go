package settings

import (
	"context"
	"player-stats-backend-go/internal/storage"
	"time"
)

type runKey struct{}

func WithRunID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, runKey{}, id)
}
func RunID(ctx context.Context) int64 { id, _ := ctx.Value(runKey{}).(int64); return id }
func (s *Service) ClaimTask(ctx context.Context, source, kind string, now time.Time) (int64, bool, error) {
	var id int64
	err := s.db.Write(ctx, func(tx *storage.Tx) error {
		result, e := tx.ExecContext(ctx, `insert into task_runs(schedule_id,run_date,run_time,created_at,server_id,server_name,task_type,task_label,status) select t.id,?,?,?,s.source_id,s.source_name,t.kind,s.source_name||'：'||t.kind,'RUNNING' from task_schedules t join log_sources s on s.source_id=t.source_id where t.source_id=? and t.kind=? and t.enabled=1 and s.enabled=1 on conflict(schedule_id,run_date,run_time) do nothing`, now.Format("2006-01-02"), now.Format("15:04"), time.Now(), source, kind)
		if e != nil {
			return e
		}
		n, e := result.RowsAffected()
		if e != nil || n == 0 {
			return e
		}
		id, e = result.LastInsertId()
		return e
	})
	return id, id > 0, err
}
func (s *Service) RecordTask(ctx context.Context, status, message, details string, scanned, success, skipped, failed int) error {
	id := RunID(ctx)
	if id == 0 {
		return nil
	}
	var finished any
	if status != "RUNNING" {
		finished = time.Now()
	}
	_, err := s.db.ExecContext(ctx, `update task_runs set status=?,message=?,file_details=?,scanned_files=?,success_files=?,skipped_files=?,failed_files=?,finished_at=? where id=?`, status, message, details, scanned, success, skipped, failed, finished, id)
	return err
}
