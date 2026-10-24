package importer

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/settings"
	"player-stats-backend-go/internal/storage"
	"testing"
	"time"
)

func TestSchedulerReadsOnlySchedulesAndDatePolicy(t *testing.T) {
	ctx := context.Background()
	cfg := config.Load()
	root := t.TempDir()
	cfg.DatabasePath = filepath.Join(root, "player.db")
	cfg.LiveLogSnapshotDirectory = filepath.Join(root, "live")
	db, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = storage.Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e = storage.EnsureDefaults(ctx, db, cfg); e != nil {
		t.Fatal(e)
	}
	// The scheduler must not read unrelated sharing settings or require an SMB
	// connection simply to inspect disabled schedules.
	if _, e = db.ExecContext(ctx, `delete from sharing_settings`); e != nil {
		t.Fatal(e)
	}
	policy := settings.NewService(db, cfg)
	service := NewService(db, cfg, policy)
	defer service.CloseLiveLogSnapshots()
	var messages bytes.Buffer
	scheduler := NewAutoTaskScheduler(policy, service, slog.New(slog.NewTextHandler(&messages, nil)), cfg.Location)
	defer scheduler.cancel()
	scheduler.check(time.Now().In(cfg.Location))
	if messages.Len() != 0 {
		t.Fatalf("scheduler loaded unrelated settings: %s", messages.String())
	}
}
