package importer

import (
	"context"
	"fmt"
	"time"
)

// Rebuild visits every local CSV, independent of source enablement and the
// interactive per-run budget. Each file remains an independent transaction.
func (s *Service) Rebuild(ctx context.Context, listener ProgressListener) error {
	if _, err := s.db.ExecContext(ctx, `update database_metadata set rebuild_required=1 where id=1`); err != nil {
		return err
	}
	for _, source := range s.cfg.ArchiveSources() {
		if saved, ok, e := s.settings.SourceByID(ctx, source.ID); e != nil {
			return e
		} else if ok {
			source.Name = saved.SourceName
			source.FileGlob = saved.SMBFileGlob
		}
		files, e := s.localFiles(source)
		if e != nil {
			return e
		}
		for _, file := range files {
			if e := ctx.Err(); e != nil {
				return e
			}
			status, e := s.importFileStatus(ctx, source, file, true)
			if e != nil {
				return e
			}
			if status.Status == "SKIPPED_TODAY" || status.Status == "IMPORTED" {
				continue
			}
			if listener != nil {
				listener.FileStarted(source.ID, source.Name, file)
			}
			result := s.importOne(ctx, source, file, true)
			if listener != nil {
				listener.FileFinished(result)
			}
			if result.Status == "FAILED" {
				return fmt.Errorf("rebuild %s/%s: %s", source.ID, file.FileName, result.Message)
			}
		}
	}
	if err := s.db.Check(ctx); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `update database_metadata set rebuild_required=0 where id=1`)
	return err
}

// RecoverTaskRuns is called only after the exclusive process lock is held.
func (s *Service) RecoverTaskRuns(ctx context.Context) error {
	_, e := s.db.ExecContext(ctx, `update task_runs set status='INTERRUPTED',finished_at=?,message='服务中断，任务未完成' where status='RUNNING'`, time.Now())
	return e
}
