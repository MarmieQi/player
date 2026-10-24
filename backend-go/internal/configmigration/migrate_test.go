package configmigration

import (
	"context"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"os"
	"path/filepath"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/settings"
	"player-stats-backend-go/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestReadOnlyLegacyConfigurationMigration(t *testing.T) {
	dsn := os.Getenv("PLAYER_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("MySQL reader integration requires the disposable CI MySQL fixture")
	}
	src, e := sql.Open("mysql", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer src.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var name string
	if e = src.QueryRowContext(ctx, `select database()`).Scan(&name); e != nil || !strings.HasSuffix(name, "_test") {
		t.Fatal("refusing non-test database", e)
	}
	for _, table := range []string{"admin_users", "sync_config", "sync_source_config", "group_chat_copy_config", "runtime_config"} {
		if _, e = src.ExecContext(ctx, `drop table if exists `+table); e != nil {
			t.Fatal(e)
		}
	}
	for _, stmt := range []string{
		`create table admin_users(id bigint primary key,username varchar(64),password_hash varchar(120),enabled boolean,created_at datetime(6),updated_at datetime(6))`,
		`insert into admin_users values(7,'fixture-admin','fixture-hash',1,'2026-01-01 00:00:00','2026-01-01 00:00:00')`,
		`create table sync_config(id int primary key,smb_host varchar(255),smb_password varchar(255),skip_today boolean,astrbot_api_key varchar(128),share_ttl_minutes int,auto_run boolean)`,
		`insert into sync_config values(1,'fixture-nas','fixture-secret',1,'fixture-plugin-key',45,0)`,
		`create table sync_source_config(id int,source_id varchar(64),source_name varchar(100),smb_directory varchar(255),enabled boolean)`,
		`insert into sync_source_config values(1,'main','主服','logs/main',1),(2,'sub','2服','logs/sub',1)`,
	} {
		if _, e = src.ExecContext(ctx, stmt); e != nil {
			t.Fatal(e)
		}
	}
	cfg := config.Load()
	cfg.LiveLogSnapshotTTLSeconds = 60
	out := filepath.Join(t.TempDir(), "player.db")
	if e = Migrate(ctx, src, out, cfg); e != nil {
		t.Fatal(e)
	}
	if e = Migrate(ctx, src, out, cfg); e == nil {
		t.Fatal("existing target overwritten")
	}
	cfg.DatabasePath = out
	dst, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer dst.Close()
	policy := settings.NewService(dst, cfg)
	connection, e := policy.SMBConnectionConfig(ctx)
	if e != nil || connection.Host != "fixture-nas" || connection.Password != "fixture-secret" {
		t.Fatalf("source connection lost: %+v %v", connection, e)
	}
	runtime, e := policy.Runtime(ctx)
	if e != nil || runtime.Saved.LiveCacheSeconds != 60 {
		t.Fatal("legacy environment defaults lost", e)
	}
	required, e := storage.RequiresRebuild(ctx, dst)
	if e != nil || !required {
		t.Fatal("migration did not require regeneration", e)
	}
	var hash string
	if e = dst.QueryRowContext(ctx, `select password_hash from admin_users where id=7`).Scan(&hash); e != nil || hash != "fixture-hash" {
		t.Fatal("administrator hash changed", e)
	}
	var count int
	if e = src.QueryRowContext(ctx, `select count(*) from admin_users`).Scan(&count); e != nil || count != 1 {
		t.Fatal("source was changed", e)
	}
	var imported int
	if e = dst.QueryRowContext(ctx, `select count(*) from import_files`).Scan(&imported); e != nil || imported != 0 {
		t.Fatal("generated data unexpectedly migrated", e)
	}
	// A failed conversion never publishes a destination or damages the existing one.
	if _, e = src.ExecContext(ctx, `insert into admin_users values(8,'FIXTURE-ADMIN','other',1,now(),now())`); e != nil {
		t.Fatal(e)
	}
	broken := filepath.Join(t.TempDir(), "failed.db")
	if e = Migrate(ctx, src, broken, cfg); e == nil {
		t.Fatal("conflicting normalized accounts accepted")
	}
	if _, e = os.Stat(broken); !os.IsNotExist(e) {
		t.Fatal("failed conversion published database")
	}
}
