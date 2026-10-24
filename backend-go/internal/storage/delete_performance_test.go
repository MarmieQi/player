package storage_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"player-stats-backend-go/internal/importer"
	"player-stats-backend-go/internal/settings"
	"player-stats-backend-go/internal/stats"
)

// Check work performed, not a machine-dependent timeout: unaffected historical
// daily cells must never be deleted/reinserted when removing other dates.
func TestBatchDeletionOnlyRebuildsAffectedDates(t *testing.T) {
	db, cfg := fixture(t)
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	queries := []string{
		`with recursive n(i) as (values(1) union all select i+1 from n where i<80) insert into players(account_key,account_name) select 'player'||i,'Player'||i from n`,
		`with recursive n(i) as (values(0) union all select i+1 from n where i<364) insert into import_files(source_id,relative_path,log_date,file_size,last_modified,content_hash,parser_version,imported_at,row_count,ignored_count) select 'main','player_actions_'||date('2025-01-01','+'||i||' days')||'.csv',date('2025-01-01','+'||i||' days'),0,1,'fixture'||i,1,1,80,0 from n`,
		`insert into file_player_daily select f.id,p.id,f.log_date,1,0 from import_files f cross join players p`,
		`insert into file_player_presence select f.id,p.id,f.id*1000000 from import_files f cross join players p`,
		`insert into player_daily_stats select f.source_id,d.player_id,d.stat_date,sum(d.broken_count),sum(d.placed_count) from file_player_daily d join import_files f on f.id=d.file_id group by f.source_id,d.player_id,d.stat_date`,
		`insert into player_profiles select f.source_id,d.player_id,min(d.first_seen_at) from file_player_presence d join import_files f on f.id=d.file_id group by f.source_id,d.player_id`,
		`create table deletion_test_counts(deletes integer not null,inserts integer not null) strict`,
		`insert into deletion_test_counts values(0,0)`,
		`create trigger deletion_test_delete after delete on player_daily_stats begin update deletion_test_counts set deletes=deletes+1;end`,
		`create trigger deletion_test_insert after insert on player_daily_stats begin update deletion_test_counts set inserts=inserts+1;end`,
	}
	for _, q := range queries {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	request := importer.DeleteImportRecordsRequest{}
	day := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 30; i++ {
		request.Files = append(request.Files, importer.ImportRecordKey{ServerID: "main", RemotePath: fmt.Sprintf("player_actions_%s.csv", day.AddDate(0, 0, i).Format("2006-01-02"))})
	}
	svc := importer.NewService(db, cfg, settings.NewService(db, cfg))
	defer svc.CloseLiveLogSnapshots()
	started := time.Now()
	result := svc.DeleteImportRecords(ctx, request)
	t.Logf("365 files x 80 players; delete 30 files: %s", time.Since(started))
	if len(result) != 30 {
		t.Fatal(result)
	}
	for _, r := range result {
		if !r.Deleted {
			t.Fatal(r)
		}
	}
	var deletes, inserts int
	if err := db.QueryRowContext(ctx, `select deletes,inserts from deletion_test_counts`).Scan(&deletes, &inserts); err != nil {
		t.Fatal(err)
	}
	if deletes != 30*80 || inserts != 0 {
		t.Fatalf("untouched dates rebuilt: deletes=%d inserts=%d", deletes, inserts)
	}
	total, err := stats.NewService(db, cfg).Overview(ctx, "main", nil, nil)
	if err != nil || total.BrokenCount != 335*80 {
		t.Fatalf("bad remaining totals: %+v %v", total, err)
	}
	var first int64
	if err := db.QueryRowContext(ctx, `select min(first_seen_at) from player_profiles`).Scan(&first); err != nil || first != 31000000 {
		t.Fatalf("bad first appearance: %d %v", first, err)
	}
	if err := db.Check(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeletionRebuildsSharedDateAndKeepsOtherSourceUntouched(t *testing.T) {
	db, cfg := fixture(t)
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	queries := []string{
		`insert into players values(1,'alex','Alex'),(2,'bob','Bob')`,
		`insert into import_files values(1,'main','one.csv','2026-04-01',0,1,'a',1,1,1,0),(2,'main','two.csv','2026-04-01',0,1,'b',1,1,1,0),(3,'sub','sub.csv','2026-04-01',0,1,'c',1,1,1,0),(4,'main','later.csv','2026-04-03',0,1,'d',1,1,1,0)`,
		`insert into file_player_daily values(1,1,'2026-04-01',5,0),(1,2,'2026-04-02',2,0),(2,1,'2026-04-01',7,0),(3,1,'2026-04-01',99,0),(4,1,'2026-04-03',50,0)`,
		`insert into file_player_presence values(1,1,100),(1,2,100),(2,1,200),(3,1,50),(4,1,400)`,
		`insert into player_daily_stats select f.source_id,d.player_id,d.stat_date,sum(d.broken_count),sum(d.placed_count) from file_player_daily d join import_files f on f.id=d.file_id group by f.source_id,d.player_id,d.stat_date`,
		`insert into player_profiles select f.source_id,d.player_id,min(d.first_seen_at) from file_player_presence d join import_files f on f.id=d.file_id group by f.source_id,d.player_id`,
		`create trigger protect_unaffected_daily before delete on player_daily_stats when old.source_id='sub' or old.stat_date='2026-04-03' begin select raise(abort,'unrelated daily cell touched');end`,
	}
	for _, q := range queries {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	svc := importer.NewService(db, cfg, settings.NewService(db, cfg))
	defer svc.CloseLiveLogSnapshots()
	result, err := svc.DeleteImportRecord(ctx, "main", "one.csv")
	if err != nil || !result.Deleted {
		t.Fatalf("delete failed: %+v %v", result, err)
	}
	for _, tc := range []struct {
		source, date string
		want         int
	}{{"main", "2026-04-01", 7}, {"main", "2026-04-03", 50}, {"sub", "2026-04-01", 99}} {
		var value int
		if err := db.QueryRowContext(ctx, `select sum(broken_count) from player_daily_stats where source_id=? and stat_date=?`, tc.source, tc.date).Scan(&value); err != nil || value != tc.want {
			t.Fatalf("%+v got %d %v", tc, value, err)
		}
	}
	var first int64
	if err := db.QueryRowContext(ctx, `select first_seen_at from player_profiles where source_id='main' and player_id=1`).Scan(&first); err != nil || first != 200 {
		t.Fatalf("main first %d %v", first, err)
	}
	if err := db.QueryRowContext(ctx, `select first_seen_at from player_profiles where source_id='sub' and player_id=1`).Scan(&first); err != nil || first != 50 {
		t.Fatalf("sub first %d %v", first, err)
	}
	var bob int
	if err := db.QueryRowContext(ctx, `select count(*) from player_profiles where source_id='main' and player_id=2`).Scan(&bob); err != nil || bob != 0 {
		t.Fatal("removed player's profile survived", err)
	}
	if err := db.Check(ctx); err != nil {
		t.Fatal(err)
	}
}
