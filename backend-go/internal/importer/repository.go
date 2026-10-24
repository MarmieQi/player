package importer

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/storage"
	"time"
)

const parserVersion = 1

func (s *Service) findImportFile(ctx context.Context, serverID, remotePath string) (importedFileRecord, bool, error) {
	return scanImportFile(s.db.QueryRowContext(ctx, importFileSelectQuery()+` where server_id=? and remote_path=?`, serverID, filepath.Base(remotePath)))
}
func findImportFileTx(ctx context.Context, tx *storage.Tx, serverID, remotePath string) (importedFileRecord, bool, error) {
	return scanImportFile(tx.QueryRowContext(ctx, importFileSelectQuery()+` where server_id=? and remote_path=?`, serverID, filepath.Base(remotePath)))
}
func scanImportFile(row *storage.Row) (importedFileRecord, bool, error) {
	var r importedFileRecord
	e := row.Scan(&r.id, &r.serverID, &r.serverName, &r.remotePath, &r.fileName, &r.logDate, &r.fileSize, &r.lastModified, &r.contentHash, &r.importedAt, &r.rowCount, &r.ignoredCount, &r.parserVersion)
	if errors.Is(e, sql.ErrNoRows) {
		return r, false, nil
	}
	return r, e == nil, e
}
func importFileSelectQuery() string {
	return `select id,server_id,server_name,remote_path,file_name,log_date,file_size,last_modified,content_hash,imported_at,row_count,ignored_count,parser_version from v_import_files`
}
func upsertImportFile(ctx context.Context, tx *storage.Tx, existingID int64, source config.Source, file RemoteLogFile, logDate *time.Time, parsed parsedLogFile) (int64, error) {
	var date any
	if logDate != nil {
		date = storage.Date(*logDate)
	}
	_, e := tx.ExecContext(ctx, `insert into import_files(source_id,relative_path,log_date,file_size,last_modified,content_hash,parser_version,imported_at,row_count,ignored_count) values(?,?,?,?,?,?,?,?,?,?) on conflict(source_id,relative_path) do update set log_date=excluded.log_date,file_size=excluded.file_size,last_modified=excluded.last_modified,content_hash=excluded.content_hash,parser_version=excluded.parser_version,imported_at=excluded.imported_at,row_count=excluded.row_count,ignored_count=excluded.ignored_count`, source.ID, file.FileName, date, file.Size, file.LastModified, parsed.contentHash, parserVersion, time.Now(), parsed.rowCount, parsed.ignoredCount)
	if e != nil {
		return 0, e
	}
	var id int64
	e = tx.QueryRowContext(ctx, `select id from import_files where source_id=? and relative_path=?`, source.ID, file.FileName).Scan(&id)
	return id, e
}
func loadFileSeenPlayers(ctx context.Context, tx *storage.Tx, id int64) (map[string]struct{}, error) {
	rows, e := tx.QueryContext(ctx, `select player_name from v_file_presence where import_file_id=?`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var name string
		if e := rows.Scan(&name); e != nil {
			return nil, e
		}
		out[name] = struct{}{}
	}
	return out, rows.Err()
}
func deleteFileDetails(ctx context.Context, tx *storage.Tx, id int64) error {
	for _, table := range []string{"file_player_daily", "file_player_presence", "file_player_resources", "file_player_milestones"} {
		if _, e := tx.ExecContext(ctx, `delete from `+table+` where file_id=?`, id); e != nil {
			return e
		}
	}
	return nil
}
func playerID(ctx context.Context, tx *storage.Tx, name string) (int64, error) {
	_, e := tx.ExecContext(ctx, `insert into players(account_key,account_name) values(account_key(?),?) on conflict(account_key) do nothing`, name, name)
	if e != nil {
		return 0, e
	}
	var id int64
	e = tx.QueryRowContext(ctx, `select id from players where account_key=account_key(?)`, name).Scan(&id)
	return id, e
}
func insertParsedDetails(ctx context.Context, tx *storage.Tx, fileID int64, p parsedLogFile) error {
	ids := map[string]int64{}
	lookup := func(name string) (int64, error) {
		if id := ids[name]; id > 0 {
			return id, nil
		}
		id, e := playerID(ctx, tx, name)
		ids[name] = id
		return id, e
	}
	for k, c := range p.stats {
		if c.broken == 0 && c.placed == 0 {
			continue
		}
		id, e := lookup(k.playerName)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `insert into file_player_daily values(?,?,?,?,?) on conflict(file_id,player_id,stat_date) do update set broken_count=broken_count+excluded.broken_count,placed_count=placed_count+excluded.placed_count`, fileID, id, storage.Date(k.statDate), c.broken, c.placed); e != nil {
			return e
		}
	}
	for name, first := range p.firstSeenByPlayer {
		id, e := lookup(name)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `insert into file_player_presence values(?,?,?) on conflict(file_id,player_id) do update set first_seen_at=min(first_seen_at,excluded.first_seen_at)`, fileID, id, first); e != nil {
			return e
		}
	}
	for _, kind := range []struct {
		category string
		counts   map[typedPlayerKey]int64
	}{{"ore", p.oreCounts}, {"wood", p.woodCounts}, {"sapling", p.saplingCounts}} {
		for k, c := range kind.counts {
			if c <= 0 {
				continue
			}
			id, e := lookup(k.playerName)
			if e != nil {
				return e
			}
			if _, e = tx.ExecContext(ctx, `insert into file_player_resources values(?,?,?,?,?) on conflict(file_id,player_id,category,resource_type) do update set resource_count=resource_count+excluded.resource_count`, fileID, id, kind.category, k.typ, c); e != nil {
				return e
			}
		}
	}
	for k, v := range p.milestones {
		id, e := lookup(k.playerName)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `insert into file_player_milestones values(?,?,?,?,?) on conflict(file_id,player_id,milestone_type) do update set first_seen_at=min(first_seen_at,excluded.first_seen_at),detail=case when excluded.first_seen_at<first_seen_at then excluded.detail else detail end`, fileID, id, k.typ, v.firstSeenAt, v.detail); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) syncProfiles(ctx context.Context, tx *storage.Tx, sourceID, sourceName string, names map[string]struct{}) error {
	seen := map[int64]bool{}
	for name := range names {
		var id int64
		e := tx.QueryRowContext(ctx, `select id from players where account_key=account_key(?)`, name).Scan(&id)
		if e == sql.ErrNoRows {
			continue
		}
		if e != nil {
			return e
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		for _, table := range []string{"player_daily_stats", "player_profiles"} {
			if _, e := tx.ExecContext(ctx, `delete from `+table+` where source_id=? and player_id=?`, sourceID, id); e != nil {
				return e
			}
		}
		if _, e = tx.ExecContext(ctx, `insert into player_daily_stats select f.source_id,d.player_id,d.stat_date,sum(d.broken_count),sum(d.placed_count) from file_player_daily d join import_files f on f.id=d.file_id where f.source_id=? and d.player_id=? group by f.source_id,d.player_id,d.stat_date`, sourceID, id); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `insert into player_profiles select f.source_id,d.player_id,min(d.first_seen_at) from file_player_presence d join import_files f on f.id=d.file_id where f.source_id=? and d.player_id=? group by f.source_id,d.player_id`, sourceID, id); e != nil {
			return e
		}
	}
	return nil
}

func (s *Service) hasImportedDetailRows(ctx context.Context, importFileID int64) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		select (
			(select count(*) from v_file_presence where import_file_id = ?) +
			(select count(*) from v_file_daily where import_file_id = ?)
		)
	`, importFileID, importFileID).Scan(&count)
	return count > 0, err
}
