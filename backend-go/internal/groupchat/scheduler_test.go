package groupchat

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"player-stats-backend-go/internal/importer"
	"testing"
	"time"
)

type fakeScheduledCopy struct {
	schedule      CopySchedule
	sourceEnabled bool
	starts        int
	skip          bool
	startError    error
	records       int
}

func (f *fakeScheduledCopy) Schedule(context.Context) (CopySchedule, error) { return f.schedule, nil }
func (f *fakeScheduledCopy) Config(context.Context) (CopyConfig, error) {
	return CopyConfig{Enabled: f.sourceEnabled}, nil
}
func (f *fakeScheduledCopy) Start(_ context.Context, skip bool) (importer.ImportJobView, error) {
	f.starts++
	f.skip = skip
	return importer.ImportJobView{JobID: "test"}, f.startError
}
func (f *fakeScheduledCopy) Job(string) (importer.ImportJobView, error) {
	return importer.ImportJobView{Status: "FINISHED"}, nil
}
func (f *fakeScheduledCopy) recordScheduled(context.Context, importer.ImportJobView, string) error {
	f.records++
	return nil
}

func TestChatScheduleRunsOncePerDayAndHonorsSource(t *testing.T) {
	f := &fakeScheduledCopy{schedule: CopySchedule{Enabled: true, Time: "00:10", SkipToday: true}, sourceEnabled: true}
	zone := time.FixedZone("UTC+8", 8*3600)
	s := &Scheduler{service: f, logger: slog.New(slog.NewTextHandler(io.Discard, nil)), location: zone, seen: map[string]bool{}}
	// Cancelled monitor context keeps this gate test deterministic without sleeps.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	now := time.Date(2026, 10, 23, 0, 10, 0, 0, zone)
	s.check(ctx, now.Add(-time.Minute))
	if f.starts != 0 {
		t.Fatal("ran before scheduled minute")
	}
	s.check(ctx, now)
	s.check(ctx, now.Add(30*time.Second))
	if f.starts != 1 || !f.skip {
		t.Fatalf("starts=%d skip=%v", f.starts, f.skip)
	}
	// Changing the shared date policy must apply to the next scheduled run.
	f.schedule.SkipToday = false
	s.check(ctx, now.AddDate(0, 0, 1))
	if f.starts != 2 || f.skip {
		t.Fatalf("next day did not follow updated policy: starts=%d skip=%v", f.starts, f.skip)
	}
	f.sourceEnabled = false
	s.check(ctx, now.AddDate(0, 0, 2))
	if f.starts != 2 {
		t.Fatal("disabled source copied")
	}
	f.sourceEnabled = true
	f.schedule.Enabled = false
	s.check(ctx, now.AddDate(0, 0, 2))
	if f.starts != 2 {
		t.Fatal("disabled schedule copied")
	}
}

func TestChatScheduleConflictIsLoggedWithoutRetryStorm(t *testing.T) {
	f := &fakeScheduledCopy{schedule: CopySchedule{Enabled: true, Time: "00:10"}, sourceEnabled: true, startError: errors.New("copy already running")}
	s := &Scheduler{service: f, logger: slog.New(slog.NewTextHandler(io.Discard, nil)), seen: map[string]bool{}}
	now := time.Date(2026, 10, 23, 0, 10, 0, 0, time.UTC)
	s.check(context.Background(), now)
	s.check(context.Background(), now)
	if f.starts != 1 || f.records != 1 {
		t.Fatalf("starts=%d logs=%d", f.starts, f.records)
	}
}

func TestNormalizeChatCopySchedule(t *testing.T) {
	for _, value := range []string{"", "9:30", "24:00", "12:60", "12:30:00"} {
		if _, err := normalizeSchedule(CopySchedule{Time: value}); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	if _, err := normalizeSchedule(CopySchedule{Time: "23:59"}); err != nil {
		t.Fatal(err)
	}
}
