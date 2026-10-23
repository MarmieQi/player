package storage_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/groupchat"
	"player-stats-backend-go/internal/importer"
	"player-stats-backend-go/internal/settings"
	"player-stats-backend-go/internal/storage"
)

// CI uses a disposable MySQL 8.4 database. Never target a production database.
func TestMySQLMigrationAndSharedChatSchedule(t *testing.T) {
	dsn := os.Getenv("PLAYER_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("PLAYER_TEST_MYSQL_DSN not set; MySQL integration runs in CI")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	var database string
	if err := db.QueryRowContext(ctx, "select database()").Scan(&database); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(database, "_test") {
		t.Fatal("refusing integration DDL outside a database ending in _test")
	}
	cfg := config.Load()
	cfg.LogArchiveDirectory = t.TempDir()
	cfg.LiveLogSnapshotDirectory = t.TempDir()
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := storage.EnsureDefaults(ctx, db, cfg); err != nil {
		t.Fatal(err)
	}
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatalf("migration not idempotent: %v", err)
	}
	policy := settings.NewService(db, cfg)
	initial, err := policy.GetSyncConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	initial.SkipToday = true
	if _, err := policy.SaveSyncConfig(ctx, initial); err != nil {
		t.Fatal(err)
	}
	copier := importer.NewService(db, cfg, policy)
	defer copier.CloseLiveLogSnapshots()
	chat := groupchat.NewCopyService(db, cfg, copier)
	defer chat.Close()
	if _, err := chat.SaveConfig(ctx, groupchat.CopyConfig{Enabled: true, SMBDirectory: `logs\chat`, FileGlob: "*.txt"}); err != nil {
		t.Fatal(err)
	}
	schedule, err := chat.SaveSchedule(ctx, groupchat.CopySchedule{Enabled: true, Time: "01:23"})
	if err != nil || !schedule.SkipToday {
		t.Fatalf("default shared policy: %#v %v", schedule, err)
	}
	sync, err := policy.GetSyncConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sync.SkipToday = false
	if _, err := policy.SaveSyncConfig(ctx, sync); err != nil {
		t.Fatal(err)
	}
	schedule, err = chat.SaveSchedule(ctx, groupchat.CopySchedule{Enabled: true, Time: "01:24", SkipToday: true})
	if err != nil || schedule.SkipToday {
		t.Fatalf("TXT overwrote shared policy: %#v %v", schedule, err)
	}
	// Reproduce the earlier development schema, then verify upgrade preserves paths.
	if _, err := db.ExecContext(ctx, "alter table group_chat_copy_config add column auto_skip_today boolean not null default true, drop column auto_enabled, drop column auto_time"); err != nil {
		t.Fatal(err)
	}
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatalf("legacy chat migration: %v", err)
	}
	saved, err := chat.Config(ctx)
	if err != nil || saved.SMBDirectory != "logs/chat" || !saved.Enabled {
		t.Fatalf("lost source config: %#v %v", saved, err)
	}
	schedule, err = chat.Schedule(ctx)
	if err != nil || schedule.SkipToday || schedule.Enabled || schedule.Time != "00:10" {
		t.Fatalf("legacy migration defaults: %#v %v", schedule, err)
	}
	var columns int
	if err := db.QueryRowContext(ctx, "select count(*) from information_schema.columns where table_schema=database() and table_name='group_chat_copy_config' and column_name='auto_skip_today'").Scan(&columns); err != nil || columns != 0 {
		t.Fatalf("obsolete policy column remains: %d %v", columns, err)
	}
}
