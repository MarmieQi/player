// Package configmigration is a one-shot, read-only MySQL configuration reader.
// It is never imported by the production server.
package configmigration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/storage"
	"strconv"
	"strings"
	"time"
)

type row map[string]any

func (v row) text(k string, fallback string) string {
	a, ok := v[k]
	if !ok || a == nil {
		return fallback
	}
	if b, ok := a.([]byte); ok {
		return string(b)
	}
	return fmt.Sprint(a)
}
func (v row) integer(k string, fallback int64) int64 {
	a, ok := v[k]
	if !ok || a == nil {
		return fallback
	}
	n, e := strconv.ParseInt(v.text(k, ""), 10, 64)
	if e != nil {
		return fallback
	}
	return n
}
func (v row) enabled(k string, fallback bool) bool {
	n := int64(0)
	if fallback {
		n = 1
	}
	return v.integer(k, n) != 0
}
func (v row) stamp(k string) time.Time {
	if t, ok := v[k].(time.Time); ok {
		return t
	}
	return time.Now().UTC()
}
func readTable(ctx context.Context, tx *sql.Tx, name string, required bool) ([]row, error) {
	var count int
	if e := tx.QueryRowContext(ctx, `select count(*) from information_schema.tables where table_schema=database() and table_name=?`, name).Scan(&count); e != nil {
		return nil, e
	}
	if count == 0 {
		if required {
			return nil, fmt.Errorf("required source table missing: %s", name)
		}
		return nil, nil
	}
	rows, e := tx.QueryContext(ctx, `select * from `+name)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	cols, e := rows.Columns()
	if e != nil {
		return nil, e
	}
	out := []row{}
	for rows.Next() {
		values := make([]any, len(cols))
		ptr := make([]any, len(cols))
		for i := range values {
			ptr[i] = &values[i]
		}
		if e := rows.Scan(ptr...); e != nil {
			return nil, e
		}
		v := row{}
		for i, k := range cols {
			v[k] = values[i]
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Migrate refuses an existing target. A private staging database is validated,
// checkpointed and closed before an atomic, non-overwriting publication.
func Migrate(ctx context.Context, source *sql.DB, target string, cfg config.Config) error {
	target, e := filepath.Abs(target)
	if e != nil {
		return e
	}
	if _, e = os.Lstat(target); !os.IsNotExist(e) {
		return fmt.Errorf("target already exists or cannot be inspected")
	}
	if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(target), ".config-migrate-*.db")
	if e != nil {
		return e
	}
	temp := f.Name()
	f.Close()
	defer os.Remove(temp)
	defer os.Remove(temp + "-wal")
	defer os.Remove(temp + "-shm")
	tx, e := source.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	tables := map[string][]row{}
	for _, name := range []string{"admin_users", "sync_config", "sync_source_config", "group_chat_copy_config", "runtime_config"} {
		v, e := readTable(ctx, tx, name, name == "admin_users" || name == "sync_config" || name == "sync_source_config")
		if e != nil {
			return e
		}
		tables[name] = v
	}
	if len(tables["admin_users"]) == 0 || len(tables["sync_config"]) != 1 {
		return fmt.Errorf("source lacks administrator or singleton settings")
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	cfg.DatabasePath = temp
	db, e := storage.Open(ctx, cfg)
	if e != nil {
		return e
	}
	closed := false
	defer func() {
		if !closed {
			db.Close()
		}
	}()
	if e = storage.Migrate(ctx, db); e != nil {
		return e
	}
	if e = storage.EnsureDefaults(ctx, db, cfg); e != nil {
		return e
	}
	old := tables["sync_config"][0]
	e = db.Write(ctx, func(dst *storage.Tx) error {
		for _, u := range tables["admin_users"] {
			if u.text("password_hash", "") == "" {
				return fmt.Errorf("source administrator hash missing")
			}
			if _, e := dst.ExecContext(ctx, `insert into admin_users(id,username,username_key,password_hash,enabled,created_at,updated_at) values(?,?,account_key(?),?,?,?,?)`, u.integer("id", 0), u.text("username", ""), u.text("username", ""), u.text("password_hash", ""), u.enabled("enabled", true), u.stamp("created_at"), u.stamp("updated_at")); e != nil {
				return e
			}
		}
		if _, e := dst.ExecContext(ctx, `update smb_connections set host=?,port=?,domain=?,username=?,password=?,share=? where id=1`, old.text("smb_host", ""), old.integer("smb_port", 445), old.text("smb_domain", ""), old.text("smb_username", ""), old.text("smb_password", ""), old.text("smb_share", "")); e != nil {
			return e
		}
		if _, e := dst.ExecContext(ctx, `update ingest_policy set skip_today=? where id=1`, old.enabled("skip_today", true)); e != nil {
			return e
		}
		key := old.text("astrbot_api_key", "")
		if key != "" {
			if _, e := dst.ExecContext(ctx, `update sharing_settings set astrbot_api_key=? where id=1`, key); e != nil {
				return e
			}
		}
		if _, e := dst.ExecContext(ctx, `update sharing_settings set share_ttl_minutes=?,log_query_share_ttl_minutes=? where id=1`, old.integer("share_ttl_minutes", 60), old.integer("log_query_share_ttl_minutes", old.integer("share_ttl_minutes", 60))); e != nil {
			return e
		}
		seen := map[string]bool{}
		for _, s := range tables["sync_source_config"] {
			id := s.text("source_id", "")
			if id == "server2" {
				id = "sub"
			}
			if id != "main" && id != "sub" {
				return fmt.Errorf("unsupported source identifier %q", id)
			}
			if seen[id] {
				return fmt.Errorf("duplicate normalized source %q", id)
			}
			seen[id] = true
			if _, e := dst.ExecContext(ctx, `update log_sources set source_name=?,smb_directory=?,smb_file_glob=?,smb_recursive=?,enabled=? where source_id=?`, s.text("source_name", id), strings.ReplaceAll(s.text("smb_directory", ""), "\\", "/"), s.text("smb_file_glob", "player_actions_*.csv"), s.enabled("smb_recursive", false), s.enabled("enabled", true), id); e != nil {
				return e
			}
		}
		for _, id := range []string{"main", "sub"} {
			for _, kind := range []string{"SYNC", "IMPORT"} {
				suffix := "sync"
				fallback := "00:20"
				if id == "sub" {
					fallback = "00:40"
				}
				if kind == "IMPORT" {
					suffix = "import"
					fallback = "00:30"
					if id == "sub" {
						fallback = "00:50"
					}
				}
				prefix := "auto_" + suffix + "_" + id
				if _, e := dst.ExecContext(ctx, `update task_schedules set enabled=?,run_time=? where source_id=? and kind=?`, old.enabled(prefix+"_enabled", old.enabled("auto_run", true)), old.text(prefix+"_time", fallback), id, kind); e != nil {
					return e
				}
			}
		}
		if len(tables["group_chat_copy_config"]) > 0 {
			chat := tables["group_chat_copy_config"][0]
			if _, e := dst.ExecContext(ctx, `update log_sources set enabled=?,smb_directory=?,smb_file_glob=? where source_id='group-chat'`, chat.enabled("enabled", false), strings.ReplaceAll(chat.text("smb_directory", ""), "\\", "/"), chat.text("file_glob", "*.txt")); e != nil {
				return e
			}
			if _, e := dst.ExecContext(ctx, `update task_schedules set enabled=?,run_time=? where source_id='group-chat'`, chat.enabled("auto_enabled", false), chat.text("auto_time", "00:10")); e != nil {
				return e
			}
		}
		p := cfg.RuntimeParameters()
		if len(tables["runtime_config"]) > 0 {
			r := tables["runtime_config"][0]
			p = config.RuntimeParameters{LiveCacheSeconds: int(r.integer("live_cache_seconds", 120)), MaxFilesPerRun: int(r.integer("max_files_per_run", 1000)), XrayRetainedJobs: int(r.integer("xray_retained_jobs", 1))}
		}
		if e := p.Validate(); e != nil {
			return e
		}
		if _, e := dst.ExecContext(ctx, `update runtime_settings set live_cache_seconds=?,max_files_per_run=?,xray_retained_jobs=? where id=1`, p.LiveCacheSeconds, p.MaxFilesPerRun, p.XrayRetainedJobs); e != nil {
			return e
		}
		_, e := dst.ExecContext(ctx, `update database_metadata set rebuild_required=1 where id=1`)
		return e
	})
	if e != nil {
		return fmt.Errorf("configuration conversion failed: %w", e)
	}
	if e = db.Check(ctx); e != nil {
		return e
	}
	if e = db.Checkpoint(ctx); e != nil {
		return e
	}
	if e = db.Close(); e != nil {
		return e
	}
	closed = true
	if e = os.Link(temp, target); e != nil {
		return fmt.Errorf("publish migrated database without overwrite: %w", e)
	}
	return nil
}
