package importer

import (
	"context"

	"player-stats-backend-go/internal/storage"
)

// Recompute only the player/date cells contributed by the removed file.
// All scratch data, cascades and aggregate updates share the file savepoint.
func deleteFileAndSyncAffectedSummaries(ctx context.Context, tx *storage.Tx, fileID int64, sourceID string) error {
	for _, query := range []string{
		`create temp table if not exists deleted_file_days(player_id integer not null,stat_date text not null,primary key(player_id,stat_date)) strict,without rowid`,
		`create temp table if not exists deleted_file_players(id integer primary key) strict`,
		`delete from deleted_file_days`,
		`delete from deleted_file_players`,
	} {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `insert into deleted_file_days select player_id,stat_date from file_player_daily where file_id=?`, fileID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `insert into deleted_file_players select player_id from file_player_presence where file_id=? union select player_id from file_player_daily where file_id=?`, fileID, fileID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `delete from import_files where id=?`, fileID); err != nil {
		return err
	}
	for _, query := range []string{
		`delete from player_daily_stats where source_id=? and (player_id,stat_date) in (select player_id,stat_date from deleted_file_days)`,
		`insert into player_daily_stats select f.source_id,d.player_id,d.stat_date,sum(d.broken_count),sum(d.placed_count) from deleted_file_days a join file_player_daily d on d.player_id=a.player_id and d.stat_date=a.stat_date join import_files f on f.id=d.file_id where f.source_id=? group by d.player_id,d.stat_date`,
		`delete from player_profiles where source_id=? and player_id in (select id from deleted_file_players)`,
		`insert into player_profiles select f.source_id,d.player_id,min(d.first_seen_at) from file_player_presence d join import_files f on f.id=d.file_id where f.source_id=? and d.player_id in (select id from deleted_file_players) group by d.player_id`,
	} {
		if _, err := tx.ExecContext(ctx, query, sourceID); err != nil {
			return err
		}
	}
	return nil
}
