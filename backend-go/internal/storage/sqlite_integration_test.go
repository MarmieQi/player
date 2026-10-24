package storage_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"player-stats-backend-go/internal/auth"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/importer"
	"player-stats-backend-go/internal/settings"
	"player-stats-backend-go/internal/share"
	"player-stats-backend-go/internal/stats"
	"player-stats-backend-go/internal/storage"
	"sync"
	"testing"
	"time"
)

func fixture(t *testing.T) (*storage.DB, config.Config) {
	t.Helper()
	root := t.TempDir()
	cfg := config.Load()
	cfg.DatabasePath = filepath.Join(root, "database/player.db")
	cfg.LogArchiveDirectory = filepath.Join(root, "synced-logs")
	cfg.MainDirectory = filepath.Join(cfg.LogArchiveDirectory, "main")
	cfg.SubDirectory = filepath.Join(cfg.LogArchiveDirectory, "sub")
	cfg.LiveLogSnapshotDirectory = filepath.Join(root, "temp")
	cfg.MaxFilesPerRun = 2
	db, e := storage.Open(context.Background(), cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	if e = storage.Migrate(context.Background(), db); e != nil {
		t.Fatal(e)
	}
	if e = storage.EnsureDefaults(context.Background(), db, cfg); e != nil {
		t.Fatal(e)
	}
	return db, cfg
}

func TestSQLiteConcurrentImportQueriesAndSettings(t *testing.T) {
	db, cfg := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cfg.MaxFilesPerRun = 1000
	policy := settings.NewService(db, cfg)
	svc := importer.NewService(db, cfg, policy)
	defer svc.CloseLiveLogSnapshots()
	queries := stats.NewService(db, cfg)
	for i := 1; i <= 12; i++ {
		writeCSV(t, cfg, fmt.Sprintf("2026-04-%02d", i), fmt.Sprintf("Player%d", i), 2)
	}
	if e := os.MkdirAll(cfg.SubDirectory, 0700); e != nil {
		t.Fatal(e)
	}
	sub := cfg
	sub.MainDirectory = cfg.SubDirectory
	for i := 1; i <= 12; i++ {
		writeCSV(t, sub, fmt.Sprintf("2026-04-%02d", i), fmt.Sprintf("Player%d", i), 1)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for _, id := range []string{"main", "sub"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			run, e := svc.ImportFromConfiguredSource(ctx, id, true, nil)
			if e != nil {
				errs <- e
			} else if run.FailedFiles > 0 {
				errs <- fmt.Errorf("import failures: %+v", run)
			}
		}(id)
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 8; j++ {
				if _, e := queries.Players(ctx, "", nil, nil, "", 100); e != nil {
					errs <- e
					return
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 8; i++ {
			if _, e := policy.SaveSharing(ctx, settings.SharingSettings{ShareTTLMinutes: 60 + i, LogQueryShareTTLMinutes: 60}); e != nil {
				errs <- e
				return
			}
		}
	}()
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	total, e := queries.Overview(ctx, "", nil, nil)
	if e != nil || total.BrokenCount != 36 || total.PlayerCount != 12 {
		t.Fatalf("concurrent result %+v %v", total, e)
	}
	if e = db.Check(ctx); e != nil {
		t.Fatal(e)
	}
}

func TestSQLiteFailedReplacementAndBatchSavepoints(t *testing.T) {
	db, cfg := fixture(t)
	ctx := context.Background()
	policy := settings.NewService(db, cfg)
	svc := importer.NewService(db, cfg, policy)
	defer svc.CloseLiveLogSnapshots()
	first := writeCSV(t, cfg, "2026-04-01", "Alex", 1)
	second := writeCSV(t, cfg, "2026-04-02", "Alex", 2)
	if _, e := svc.ImportFromConfiguredSource(ctx, "main", true, nil); e != nil {
		t.Fatal(e)
	}
	if _, e := db.ExecContext(ctx, `create trigger fixture_fail_daily before insert on file_player_daily begin select raise(abort,'fixture write failure'); end`); e != nil {
		t.Fatal(e)
	}
	writeCSV(t, cfg, "2026-04-01", "Alex", 4)
	run, e := svc.ImportFromConfiguredSource(ctx, "main", true, nil)
	if e != nil || run.FailedFiles != 1 {
		t.Fatalf("failure not reported %+v %v", run, e)
	}
	total, e := stats.NewService(db, cfg).Overview(ctx, "main", nil, nil)
	if e != nil || total.BrokenCount != 3 {
		t.Fatal("failed replacement changed prior statistics", e)
	}
	if _, e = db.ExecContext(ctx, `drop trigger fixture_fail_daily`); e != nil {
		t.Fatal(e)
	}
	if _, e = db.ExecContext(ctx, `create trigger fixture_fail_summary before insert on player_daily_stats when new.broken_count=2 begin select raise(abort,'fixture summary failure'); end`); e != nil {
		t.Fatal(e)
	}
	result := svc.DeleteImportRecords(ctx, importer.DeleteImportRecordsRequest{Files: []importer.ImportRecordKey{{ServerID: "main", RemotePath: first}, {ServerID: "main", RemotePath: second}}})
	if len(result) != 2 || result[0].Deleted || !result[1].Deleted {
		t.Fatalf("batch savepoint results %+v", result)
	}
	total, e = stats.NewService(db, cfg).Overview(ctx, "main", nil, nil)
	if e != nil || total.BrokenCount != 1 {
		t.Fatalf("partial deletion corrupted aggregates %+v %v", total, e)
	}
	if e = db.Check(ctx); e != nil {
		t.Fatal(e)
	}
}

func TestSQLiteShareSnapshotsAndPersistentOutbox(t *testing.T) {
	db, cfg := fixture(t)
	ctx := context.Background()
	policy := settings.NewService(db, cfg)
	svc := importer.NewService(db, cfg, policy)
	defer svc.CloseLiveLogSnapshots()
	writeCSV(t, cfg, "2026-04-01", "Älex", 2)
	if _, e := svc.ImportFromConfiguredSource(ctx, "main", true, nil); e != nil {
		t.Fatal(e)
	}
	statsSvc := stats.NewService(db, cfg)
	api := share.NewService(db, cfg, policy, statsSvc, nil, nil)
	token, e := api.CreatePlayerToken(ctx, "äLEX")
	if e != nil {
		t.Fatal(e)
	}
	details, e := api.PlayerDetails(ctx, token.Token)
	if e != nil || details.PlayerName != "Älex" {
		t.Fatalf("shared player %+v %v", details, e)
	}
	if len(details.Servers) == 0 || details.Servers[0].FirstSeenAt == nil || details.Servers[0].FirstSeenAt.Hour() != 19 {
		t.Fatalf("business timestamp shifted: %+v", details.Servers)
	}
	ranked, e := api.CreateRankingToken(ctx, "total", 10, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = api.RankingDetails(ctx, ranked.Token); e != nil {
		t.Fatal(e)
	}
	sent, e := api.SendXrayToGroup(ctx, share.XrayGroupSendRequest{ServerID: "main", PlayerName: "Älex", Player: &importer.XrayPlayerRiskView{PlayerName: "Älex"}})
	if e != nil {
		t.Fatal(e)
	}
	// Reconstructing the service represents a backend restart: pending delivery survives.
	again := share.NewService(db, cfg, policy, statsSvc, nil, nil)
	pending, e := again.PendingXrayGroupMessages(ctx, 10)
	if e != nil || len(pending) != 1 || pending[0].ID != sent.MessageID {
		t.Fatalf("outbox not durable: %+v %v", pending, e)
	}
	if _, e = again.MarkXrayGroupDelivery(ctx, sent.MessageID, true, ""); e != nil {
		t.Fatal(e)
	}
	pending, e = again.PendingXrayGroupMessages(ctx, 10)
	if e != nil || len(pending) != 0 {
		t.Fatal("delivered item still pending", e)
	}
	suggestions, e := statsSvc.PlayerSuggestions(ctx, "main", "äL")
	if e != nil || len(suggestions) != 1 || suggestions[0] != "Älex" {
		t.Fatalf("Unicode suggestions: %v %v", suggestions, e)
	}
	if e = db.Check(ctx); e != nil {
		t.Fatal(e)
	}
}

func TestSQLiteTaskClaimsConstraintsAndRebuild(t *testing.T) {
	db, cfg := fixture(t)
	ctx := context.Background()
	policy := settings.NewService(db, cfg)
	plans, e := policy.Schedules(ctx)
	if e != nil {
		t.Fatal(e)
	}
	plans.Tasks[0].SyncEnabled = true
	if _, e = policy.SaveSchedules(ctx, plans); e != nil {
		t.Fatal(e)
	}
	now := time.Date(2026, 10, 23, 0, 20, 0, 0, cfg.Location)
	id, claimed, e := policy.ClaimTask(ctx, "main", "SYNC", now)
	if e != nil || !claimed {
		t.Fatal("claim failed", e)
	}
	again := settings.NewService(db, cfg)
	if _, claimed, e = again.ClaimTask(ctx, "main", "SYNC", now); e != nil || claimed {
		t.Fatal("restart duplicated task", e)
	}
	if e = policy.RecordTask(settings.WithRunID(ctx, id), "FINISHED", "done", "", 1, 1, 0, 0); e != nil {
		t.Fatal(e)
	}
	svc := importer.NewService(db, cfg, policy)
	defer svc.CloseLiveLogSnapshots()
	if e = svc.ClearAutoTaskLogs(ctx); e != nil {
		t.Fatal(e)
	}
	if _, claimed, e = again.ClaimTask(ctx, "main", "SYNC", now); e != nil || claimed {
		t.Fatal("clearing UI logs erased dispatch claim", e)
	}
	for _, q := range []string{`insert into player_daily_stats values('main',999,'2026-01-01',1,0)`, `update runtime_settings set live_cache_seconds=0`, `insert into task_schedules(source_id,kind,run_time) values('group-chat','IMPORT','00:30')`} {
		if _, e = db.ExecContext(ctx, q); e == nil {
			t.Fatalf("invalid row accepted: %s", q)
		}
	}
	for day := 1; day <= 5; day++ {
		writeCSV(t, cfg, fmt.Sprintf("2026-04-%02d", day), "Alex", 1)
	}
	if _, e = db.ExecContext(ctx, `update database_metadata set rebuild_required=1`); e != nil {
		t.Fatal(e)
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if e = svc.Rebuild(cancelled, nil); e == nil {
		t.Fatal("rebuild ignored cancellation")
	}
	if needed, _ := storage.RequiresRebuild(ctx, db); !needed {
		t.Fatal("cancelled rebuild cleared guard")
	}
	if e = svc.Rebuild(ctx, nil); e != nil {
		t.Fatal(e)
	}
	var n int
	if e = db.QueryRowContext(ctx, `select count(*) from import_files`).Scan(&n); e != nil || n != 5 {
		t.Fatal("rebuild obeyed interactive budget", n, e)
	}
	if needed, _ := storage.RequiresRebuild(ctx, db); needed {
		t.Fatal("completed rebuild left guard set")
	}
	if e = svc.Rebuild(ctx, nil); e != nil {
		t.Fatal(e)
	}
	total, e := stats.NewService(db, cfg).Overview(ctx, "main", nil, nil)
	if e != nil || total.BrokenCount != 5 {
		t.Fatal("resumed rebuild duplicated data", e)
	}
	if e = db.Check(ctx); e != nil {
		t.Fatal(e)
	}
}
func TestSQLiteSchemaSettingsAndLogin(t *testing.T) {
	db, cfg := fixture(t)
	ctx := context.Background()
	if e := storage.Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e := db.Check(ctx); e != nil {
		t.Fatal(e)
	}
	service := settings.NewService(db, cfg)
	smb, e := service.SMB(ctx)
	if e != nil {
		t.Fatal(e)
	}
	smb.Host = "nas"
	smb.Password = "fixture"
	smb.Chat.Enabled = false
	if _, e = service.SaveSMB(ctx, smb); e != nil {
		t.Fatal(e)
	}
	plans, e := service.Schedules(ctx)
	if e != nil {
		t.Fatal(e)
	}
	plans.SkipToday = false
	plans.Chat.Enabled = true
	if _, e = service.SaveSchedules(ctx, plans); e == nil {
		t.Fatal("enabled TXT plan without source")
	}
	after, _ := service.Schedules(ctx)
	if !after.SkipToday {
		t.Fatal("partial plan save")
	}
	state, e := service.SaveRuntime(ctx, config.RuntimeParameters{LiveCacheSeconds: 60, MaxFilesPerRun: 1234, XrayRetainedJobs: 2})
	if e != nil || !state.RestartRequired {
		t.Fatalf("runtime %+v %v", state, e)
	}
	loaded, e := settings.InitializeRuntime(ctx, db, cfg)
	if e != nil || loaded.LiveLogSnapshotTTLSeconds != 60 {
		t.Fatal("restart settings", e)
	}
	cfg.AdminUsername = "测试Admin"
	cfg.AdminPassword = "fixture-password"
	a := auth.NewService(db, cfg)
	if e = a.EnsureAdmin(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = a.Login(ctx, auth.LoginRequest{Username: "测试admin", Password: "fixture-password"}); e != nil {
		t.Fatal(e)
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `update ingest_policy set skip_today=0`); e != nil {
		t.Fatal(e)
	}
	// WAL readers see committed data even while the write connection is occupied.
	readCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	skip, e := service.SkipToday(readCtx)
	if e != nil || !skip {
		t.Fatalf("reader blocked/uncommitted data: %v %v", skip, e)
	}
	waitCtx, stop := context.WithTimeout(ctx, 20*time.Millisecond)
	defer stop()
	if _, e = db.ExecContext(waitCtx, `update ingest_policy set skip_today=0`); e == nil {
		t.Fatal("write waiting ignored cancellation")
	}
}

const header = "日期,时间,玩家,行为,x,y,z,维度,维度2,a,b,c,方块\n"

func writeCSV(t *testing.T, cfg config.Config, day, player string, count int) string {
	t.Helper()
	if e := os.MkdirAll(cfg.MainDirectory, 0700); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(cfg.MainDirectory, "player_actions_"+day+".csv")
	body := header
	for i := 0; i < count; i++ {
		body += fmt.Sprintf("%s,19:19:00,%s,破坏方块,0,10,0,overworld,overworld,,,,minecraft:diamond_ore\n", day, player)
	}
	if e := os.WriteFile(path, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	return path
}
func TestSQLiteImportReplaceDeleteAndProgress(t *testing.T) {
	db, cfg := fixture(t)
	ctx := context.Background()
	policy := settings.NewService(db, cfg)
	svc := importer.NewService(db, cfg, policy)
	defer svc.CloseLiveLogSnapshots()
	statsSvc := stats.NewService(db, cfg)
	path := writeCSV(t, cfg, "2026-04-01", "Alex", 2)
	writeCSV(t, cfg, "2026-04-02", "alex", 1)
	writeCSV(t, cfg, "2026-04-03", "中文玩家", 3)
	run, e := svc.ImportFromConfiguredSource(ctx, "main", true, nil)
	if e != nil || run.ImportedFiles != 2 {
		t.Fatalf("first batch %+v %v", run, e)
	}
	run, e = svc.ImportFromConfiguredSource(ctx, "main", true, nil)
	if e != nil || run.ImportedFiles != 1 {
		t.Fatalf("starved later files %+v %v", run, e)
	}
	overview, e := statsSvc.Overview(ctx, "main", nil, nil)
	if e != nil || overview.BrokenCount != 6 || overview.PlayerCount != 2 {
		t.Fatalf("overview %+v %v", overview, e)
	}
	run, e = svc.ImportFromConfiguredSource(ctx, "main", true, nil)
	if e != nil || run.ImportedFiles != 0 {
		t.Fatal("duplicate import", e)
	}
	writeCSV(t, cfg, "2026-04-01", "Alex", 4)
	run, e = svc.ImportFromConfiguredSource(ctx, "main", true, nil)
	if e != nil || run.ImportedFiles != 1 {
		t.Fatalf("replace %+v %v", run, e)
	}
	overview, e = statsSvc.Overview(ctx, "main", nil, nil)
	if e != nil || overview.BrokenCount != 8 {
		t.Fatalf("replacement aggregates %+v %v", overview, e)
	}
	day := time.Date(2026, 4, 1, 0, 0, 0, 0, cfg.Location)
	filtered, e := statsSvc.Overview(ctx, "main", &day, &day)
	if e != nil || filtered.BrokenCount != 4 {
		t.Fatalf("date filter %+v %v", filtered, e)
	}
	result, e := svc.DeleteImportRecord(ctx, "main", path)
	if e != nil || !result.Deleted {
		t.Fatalf("delete %+v %v", result, e)
	}
	overview, e = statsSvc.Overview(ctx, "main", nil, nil)
	if e != nil || overview.BrokenCount != 4 {
		t.Fatalf("delete aggregates %+v %v", overview, e)
	}
	if e = db.Check(ctx); e != nil {
		t.Fatal(e)
	}
}

func TestSQLiteArchiveProgressBeyond1000AndDisabledRebuild(t *testing.T) {
	db, cfg := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg.MaxFilesPerRun = 1000
	if e := os.MkdirAll(cfg.MainDirectory, 0700); e != nil {
		t.Fatal(e)
	}
	first := time.Date(2020, 1, 1, 0, 0, 0, 0, cfg.Location)
	for i := 0; i < 1001; i++ {
		day := first.AddDate(0, 0, i).Format("2006-01-02")
		if e := os.WriteFile(filepath.Join(cfg.MainDirectory, "player_actions_"+day+".csv"), []byte(header), 0600); e != nil {
			t.Fatal(e)
		}
	}
	svc := importer.NewService(db, cfg, settings.NewService(db, cfg))
	defer svc.CloseLiveLogSnapshots()
	for _, want := range []int{1000, 1, 0} {
		run, e := svc.ImportFromConfiguredSource(ctx, "main", true, nil)
		if e != nil || run.FailedFiles != 0 || run.ImportedFiles != want {
			t.Fatalf("expected %d imports, got %+v: %v", want, run, e)
		}
	}
	sub := cfg
	sub.MainDirectory = cfg.SubDirectory
	writeCSV(t, sub, "2026-04-01", "DisabledSource", 1)
	cfg.SubEnabled = false
	cfg.MaxFilesPerRun = 1
	offline := importer.NewService(db, cfg, settings.NewService(db, cfg))
	defer offline.CloseLiveLogSnapshots()
	if e := offline.Rebuild(ctx, nil); e != nil {
		t.Fatal(e)
	}
	var files int
	if e := db.QueryRowContext(ctx, `select count(*) from import_files`).Scan(&files); e != nil || files != 1002 {
		t.Fatalf("rebuild omitted disabled source: %d %v", files, e)
	}
}

func TestSQLiteProcessLockAndReadOnlyPool(t *testing.T) {
	db, cfg := fixture(t)
	ctx := context.Background()
	release, e := storage.ProcessLock(cfg.DatabasePath)
	if e != nil {
		t.Fatal(e)
	}
	if other, e := storage.ProcessLock(cfg.DatabasePath); e == nil {
		other()
		release()
		t.Fatal("second process lock succeeded")
	}
	release()
	release, e = storage.ProcessLock(cfg.DatabasePath)
	if e != nil {
		t.Fatal("released lock not reusable", e)
	}
	defer release()
	var id int
	if e = db.QueryRowContext(ctx, `delete from ingest_policy returning id`).Scan(&id); e == nil {
		t.Fatal("read pool accepted a write")
	}
	if e = db.Check(ctx); e != nil {
		t.Fatal(e)
	}
}

func TestSQLiteFullDatabaseRollsBackConfiguration(t *testing.T) {
	db, _ := fixture(t)
	ctx := context.Background()
	var pages int
	if e := db.QueryRowContext(ctx, `pragma page_count`).Scan(&pages); e != nil {
		t.Fatal(e)
	}
	if _, e := db.ExecContext(ctx, fmt.Sprintf(`pragma max_page_count=%d`, pages+1)); e != nil {
		t.Fatal(e)
	}
	e := db.Write(ctx, func(tx *storage.Tx) error {
		if _, e := tx.ExecContext(ctx, `update ingest_policy set skip_today=0`); e != nil {
			return e
		}
		_, e := tx.ExecContext(ctx, `update sharing_settings set astrbot_api_key=hex(zeroblob(1048576))`)
		return e
	})
	if e == nil {
		t.Fatal("capacity limit accepted oversized transaction")
	}
	var skip int
	if e := db.QueryRowContext(ctx, `select skip_today from ingest_policy`).Scan(&skip); e != nil || skip != 1 {
		t.Fatalf("failed write partially committed: %d %v", skip, e)
	}
	if _, e := db.ExecContext(ctx, `pragma max_page_count=1073741823`); e != nil {
		t.Fatal(e)
	}
	if _, e := db.ExecContext(ctx, `update ingest_policy set skip_today=0`); e != nil {
		t.Fatal("writer did not recover after full database", e)
	}
	if e := db.Check(ctx); e != nil {
		t.Fatal(e)
	}
}
