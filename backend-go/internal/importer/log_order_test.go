package importer

import (
	"context"
	"fmt"
	"path/filepath"
	"player-stats-backend-go/internal/apitype"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/storage"
	"testing"
	"time"
)

func TestAdminLogPagesUseGlobalChronologicalOrder(t *testing.T) {
	state := newLogQueryState("main", "主服", logQueryCriteria{})
	defer state.task.cancel()
	state.scannedFiles = 2
	for i := 249; i >= 0; i-- {
		day := "2026-10-12"
		if i >= 125 {
			day = "2026-10-13"
		}
		state.addRow(LogQueryRow{Date: day, Time: fmt.Sprintf("00:%02d:%02d", i%125/60, i%125%60), FileName: day + ".csv", LineNumber: int64(i + 1)})
	}
	state.markFinished()
	var previous *LogQueryRow
	for page := 1; page <= 3; page++ {
		view := state.view(page, 100)
		if view.MatchedRows != 250 || view.TotalPages != 3 {
			t.Fatalf("incorrect page summary: %+v", view)
		}
		for i := range view.Rows {
			row := view.Rows[i]
			if previous != nil && compareLogQueryRows(*previous, row) > 0 {
				t.Fatalf("time went backwards across pages: %+v then %+v", *previous, row)
			}
			previous = &row
		}
	}
}

func TestAdminSortedPagesAcrossDatesKeepSourceUnchanged(t *testing.T) {
	state := newLogQueryState("main", "主服", logQueryCriteria{})
	defer state.task.cancel()
	for i := 600; i >= 0; i-- {
		day := "2026-10-12"
		if i >= 300 {
			day = "2026-10-13"
		}
		state.addRow(LogQueryRow{Date: day, Time: "00:00:00", FileName: "logs.csv", LineNumber: int64(i + 1)})
	}
	state.markFinished()
	for _, order := range []apitype.SortOrder{apitype.SortAscending, apitype.SortDescending, apitype.SortAscending} {
		for _, size := range []int{50, 200, 500} {
			seen := 0
			for page := 1; seen < 601; page++ {
				view := state.sortedView(page, size, order)
				if view.SortOrder != order {
					t.Fatal(view.SortOrder)
				}
				for _, row := range view.Rows {
					want := int64(seen + 1)
					if order == apitype.SortDescending {
						want = int64(601 - seen)
					}
					if row.LineNumber != want {
						t.Fatalf("%s size %d index %d: %+v", order, size, seen, row)
					}
					seen++
				}
			}
			if seen != 601 {
				t.Fatal(seen)
			}
		}
	}
	if state.view(1, 200).Rows[0].LineNumber != 1 {
		t.Fatal("source snapshot changed")
	}
}

func TestPartialAndFailedLogResultsStaySortedAfterNewRows(t *testing.T) {
	state := newLogQueryState("main", "主服", logQueryCriteria{})
	defer state.task.cancel()
	state.markRunning()
	state.addRow(LogQueryRow{Date: "2026-10-13", Time: "00:00:00", LineNumber: 2})
	state.addRow(LogQueryRow{Date: "2026-10-12", Time: "00:00:00", LineNumber: 1})
	if got := state.view(1, 200); got.Rows[0].LineNumber != 1 {
		t.Fatal("running result not sorted")
	}
	state.addRow(LogQueryRow{Date: "2026-10-14", Time: "00:00:00", LineNumber: 3})
	state.markFailed("test interruption")
	got := state.sortedView(1, 200, apitype.SortDescending)
	if got.Rows[0].LineNumber != 3 || got.Rows[2].LineNumber != 1 {
		t.Fatal("partial result not globally reversed")
	}
	if state.rowsDirty {
		t.Fatal("clean snapshot remains dirty")
	}
	if state.view(1, 200).Rows[0].LineNumber != 1 {
		t.Fatal("direction changed source snapshot")
	}
}

func TestPublicLogLimitKeepsRecentRowsInChronologicalOrder(t *testing.T) {
	rows := []LogQueryRow{
		{Date: "2026-10-12", Time: "23:59:59", LineNumber: 1},
		{Date: "2026-10-13", Time: "00:00:00", LineNumber: 2},
		{Date: "2026-10-13", Time: "00:00:01", LineNumber: 3},
	}
	got := publicLogRowsForRequest(rows, PublicCoordinateLogQueryRequest{Limit: 2})
	if len(got) != 2 || got[0].LineNumber != 2 || got[1].LineNumber != 3 {
		t.Fatalf("recent subset lost chronological order: %+v", got)
	}
}

func TestMiningLogRowsUseChronologicalOrderAndFileTies(t *testing.T) {
	events := []xrayEvent{
		{row: LogQueryRow{Date: "2026-10-13", Time: "00:00:00", FileName: "b.csv", LineNumber: 2}},
		{row: LogQueryRow{Date: "2026-10-12", Time: "23:59:59", FileName: "a.csv", LineNumber: 1}},
		{row: LogQueryRow{Date: "2026-10-13", Time: "00:00:00", FileName: "b.csv", LineNumber: 1}},
		{row: LogQueryRow{Date: "2026-10-13", Time: "00:00:00", FileName: "a.csv", LineNumber: 8}},
	}
	rows := rowsFromEvents(events, context.Background())
	if len(rows) != 4 || rows[0].Date != "2026-10-12" || rows[1].FileName != "a.csv" || rows[2].LineNumber != 1 || rows[3].LineNumber != 2 {
		t.Fatalf("wrong evidence log order: %+v", rows)
	}
	player := XrayPlayerRiskView{RiskScore: 80, Evidence: []XrayEvidenceView{{Type: "FIRST", Rows: []LogQueryRow{rows[3], rows[0]}}}, RareOreRows: []LogQueryRow{rows[2], rows[1]}}
	SortXrayPlayerLogsChronologically(&player)
	if player.RiskScore != 80 || player.Evidence[0].Type != "FIRST" || player.Evidence[0].Rows[0].Date != "2026-10-12" || player.RareOreRows[0].FileName != "a.csv" {
		t.Fatal("report normalization changed analysis or failed to sort logs")
	}
}

func TestPersistedAdminXrayEvidenceUsesChronologicalOrder(t *testing.T) {
	ctx := context.Background()
	cfg := config.Load()
	cfg.DatabasePath = filepath.Join(t.TempDir(), "player.db")
	db, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = storage.Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	service := &XrayAnalysisService{importer: &Service{db: db, cfg: cfg}}
	start := time.Date(2026, 10, 12, 23, 59, 59, 0, cfg.Location)
	state := newXrayState(config.Source{ID: "main", Name: "主服"}, xrayCriteria{serverID: "main", fromTime: start, toTime: start.Add(time.Second)})
	defer state.task.cancel()
	early := LogQueryRow{Date: "2026-10-12", Time: "23:59:59", LineNumber: 1}
	late := LogQueryRow{Date: "2026-10-13", Time: "00:00:00", LineNumber: 2}
	state.markFinished([]XrayPlayerRiskView{{PlayerName: "Steve", RiskScore: 80, Evidence: []XrayEvidenceView{{Type: "TEST", Rows: []LogQueryRow{late, early}}}, RareOreRows: []LogQueryRow{late, early}}})
	if e = service.saveState(ctx, state); e != nil {
		t.Fatal(e)
	}
	got, found, e := service.loadPersistedLatest(ctx, "main")
	if e != nil || !found {
		t.Fatal("stored report missing", e)
	}
	if got.Players[0].RiskScore != 80 || got.Players[0].Evidence[0].Rows[0].LineNumber != 1 || got.Players[0].RareOreRows[0].LineNumber != 1 {
		t.Fatal("persisted evidence logs not chronological")
	}
}
