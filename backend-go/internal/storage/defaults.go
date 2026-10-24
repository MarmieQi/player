package storage

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"player-stats-backend-go/internal/config"
)

func randomKey() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return "psk_" + base64.RawURLEncoding.EncodeToString(b), nil
}
func EnsureDefaults(ctx context.Context, db *DB, cfg config.Config) error {
	key, err := randomKey()
	if err != nil {
		return err
	}
	return db.Write(ctx, func(tx *Tx) error {
		for _, stmt := range []struct {
			q string
			a []any
		}{
			{`insert into database_metadata(id,instance_id) values(1,?) on conflict(id) do nothing`, []any{key}},
			{`insert into smb_connections(id) values(1) on conflict(id) do nothing`, nil},
			{`insert into ingest_policy(id) values(1) on conflict(id) do nothing`, nil},
			{`insert into sharing_settings(id,astrbot_api_key) values(1,?) on conflict(id) do nothing`, []any{key}},
			{`insert into runtime_settings values(1,120,1000,1) on conflict(id) do nothing`, nil},
		} {
			if _, e := tx.ExecContext(ctx, stmt.q, stmt.a...); e != nil {
				return e
			}
		}
		for _, source := range []struct {
			id, name, kind, dir, glob string
			enabled                   bool
		}{{"main", cfg.MainName, "csv", "main", cfg.MainFileGlob, cfg.MainEnabled}, {"sub", cfg.SubName, "csv", "sub", cfg.SubFileGlob, cfg.SubEnabled}, {"group-chat", "群服聊天", "txt", "chat", "*.txt", false}} {
			if source.name == "" {
				source.name = source.id
			}
			if source.glob == "" {
				source.glob = "player_actions_*.csv"
			}
			if _, e := tx.ExecContext(ctx, `insert into log_sources(source_id,source_name,kind,local_subdirectory,smb_file_glob,enabled) values(?,?,?,?,?,?) on conflict(source_id) do nothing`, source.id, source.name, source.kind, source.dir, source.glob, source.enabled); e != nil {
				return e
			}
		}
		for _, s := range []struct{ id, kind, t string }{{"main", "SYNC", "00:20"}, {"main", "IMPORT", "00:30"}, {"sub", "SYNC", "00:40"}, {"sub", "IMPORT", "00:50"}, {"group-chat", "SYNC", "00:10"}} {
			if _, e := tx.ExecContext(ctx, `insert into task_schedules(source_id,kind,run_time) values(?,?,?) on conflict(source_id,kind) do nothing`, s.id, s.kind, s.t); e != nil {
				return e
			}
		}
		return nil
	})
}
func InstanceID(ctx context.Context, db *DB) (string, error) {
	var id string
	e := db.QueryRowContext(ctx, `select instance_id from database_metadata where id=1`).Scan(&id)
	return id, e
}
func RequiresRebuild(ctx context.Context, db *DB) (bool, error) {
	var v bool
	e := db.QueryRowContext(ctx, `select rebuild_required from database_metadata where id=1`).Scan(&v)
	return v, e
}
