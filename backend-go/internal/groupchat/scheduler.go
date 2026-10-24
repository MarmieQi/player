package groupchat

import (
	"context"
	"log/slog"
	"player-stats-backend-go/internal/importer"
	"time"
)

type scheduledCopier interface {
	Schedule(context.Context) (CopySchedule, error)
	Config(context.Context) (CopyConfig, error)
	Start(context.Context, bool) (importer.ImportJobView, error)
	Job(string) (importer.ImportJobView, error)
	recordScheduled(context.Context, importer.ImportJobView, string) error
}

// Scheduler has its own schedule; manual and scheduled copies share one copy lock.
type Scheduler struct {
	service  scheduledCopier
	logger   *slog.Logger
	location *time.Location
	day      string
	seen     map[string]bool
}

func NewScheduler(service *CopyService, logger *slog.Logger, location *time.Location) *Scheduler {
	return &Scheduler{service: service, logger: logger, location: location, seen: map[string]bool{}}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	s.check(ctx, time.Now().In(s.location))
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.check(ctx, now.In(s.location))
		}
	}
}

func (s *Scheduler) check(ctx context.Context, now time.Time) {
	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	schedule, err := s.service.Schedule(readCtx)
	if err != nil {
		s.logger.Warn("chat copy schedule load failed", "error", err)
		return
	}
	if !schedule.Enabled || schedule.Time != now.Format("15:04") {
		return
	}
	source, err := s.service.Config(readCtx)
	if err != nil {
		s.logger.Warn("chat copy config load failed", "error", err)
		return
	}
	if !source.Enabled {
		return
	}
	day := now.Format("2006-01-02")
	if s.day != day {
		s.day = day
		s.seen = map[string]bool{}
	}
	if s.seen[schedule.Time] {
		return
	}
	if claim, ok := s.service.(interface {
		claimScheduled(context.Context, time.Time) (context.Context, bool, error)
	}); ok {
		claimedCtx, claimed, err := claim.claimScheduled(ctx, now)
		if err != nil {
			s.logger.Warn("chat task claim failed", "error", err)
			return
		}
		if !claimed {
			return
		}
		ctx = claimedCtx
	}
	s.seen[schedule.Time] = true
	job, err := s.service.Start(readCtx, schedule.SkipToday)
	if err != nil {
		s.record(ctx, importer.ImportJobView{Status: "FAILED"}, "自动同步 TXT 未启动："+err.Error())
		return
	}
	s.record(ctx, importer.ImportJobView{Status: "RUNNING"}, "自动同步 TXT 已开始")
	go s.monitor(ctx, job.JobID)
}

func (s *Scheduler) monitor(ctx context.Context, id string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job, err := s.service.Job(id)
			if err != nil {
				s.record(ctx, importer.ImportJobView{Status: "FAILED"}, err.Error())
				return
			}
			if job.Status != "PENDING" && job.Status != "RUNNING" {
				s.record(ctx, job, job.Message)
				return
			}
		}
	}
}
func (s *Scheduler) record(ctx context.Context, job importer.ImportJobView, message string) {
	logCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.service.recordScheduled(logCtx, job, message); err != nil {
		s.logger.Warn("chat copy task log failed", "error", err)
	}
}
