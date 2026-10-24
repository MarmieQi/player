package share

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/importer"
	"player-stats-backend-go/internal/storage"
	"slices"
	"testing"
	"time"
)

func TestLogSnapshotsDisplayAscendingAcrossPersistedPageBoundaries(t *testing.T) {
	for _, mode := range []string{"ascending-pages", "descending-pages", "descending-inline"} {
		t.Run(mode, func(t *testing.T) {
			cfg := config.Load()
			cfg.DatabasePath = filepath.Join(t.TempDir(), "player.db")
			ctx := context.Background()
			db, e := storage.Open(ctx, cfg)
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			if e = storage.Migrate(ctx, db); e != nil {
				t.Fatal(e)
			}
			canonical := make([]importer.LogQueryRow, 430)
			base := time.Date(2026, 10, 12, 23, 59, 0, 0, cfg.Location)
			for i := range canonical {
				at := base.Add(time.Duration(i) * time.Second)
				canonical[i] = importer.LogQueryRow{Date: at.Format("2006-01-02"), Time: at.Format("15:04:05"), LineNumber: int64(i + 1), FileName: "log.csv", FilePath: "private/path"}
			}
			stored := slices.Clone(canonical)
			if mode != "ascending-pages" {
				slices.Reverse(stored)
			}
			metadata := logQuerySnapshotMetadata{PublicCoordinateLogQueryView: importer.PublicCoordinateLogQueryView{DisplayedRows: len(stored), MatchedRows: int64(len(stored))}}
			if mode == "ascending-pages" {
				metadata.RowOrder = "asc"
			}
			if mode == "descending-inline" {
				metadata.Rows = stored
			}
			payload, e := json.Marshal(metadata)
			if e != nil {
				t.Fatal(e)
			}
			now := time.Now()
			if _, e = db.ExecContext(ctx, `insert into share_links(kind,token,created_at,expires_at,payload_json) values('log','test',?,?,?)`, now, now.Add(time.Hour), string(payload)); e != nil {
				t.Fatal(e)
			}
			if mode != "descending-inline" {
				for start := 0; start < len(stored); start += 200 {
					end := min(start+200, len(stored))
					compressed, e := compressLogQueryRows(stored[start:end])
					if e != nil {
						t.Fatal(e)
					}
					if _, e = db.ExecContext(ctx, `insert into share_pages values(?,?,?,?)`, "test", start/200+1, end-start, compressed); e != nil {
						t.Fatal(e)
					}
				}
			}
			service := NewService(db, cfg, nil, nil, nil, nil)
			for _, size := range []int{50, 100, 200} {
				for page := 1; page <= (len(stored)+size-1)/size; page++ {
					result, e := service.LogQueryDetails(ctx, "test", page, size)
					if e != nil {
						t.Fatal(e)
					}
					if result.TotalRows != len(stored) {
						t.Fatal("wrong total rows")
					}
					for i, row := range result.Rows {
						expected := canonical[(page-1)*size+i]
						if row.Date != expected.Date || row.Time != expected.Time || row.LineNumber != expected.LineNumber || row.FilePath != "" {
							t.Fatalf("%s size=%d page=%d row=%d: got %+v, expected %+v", mode, size, page, i, row, expected)
						}
					}
				}
			}
			// Sort metadata stays internal to storage rather than changing API output.
			result, e := service.LogQueryDetails(ctx, "test", 1, 50)
			if e != nil {
				t.Fatal(e)
			}
			public, e := json.Marshal(result)
			if e != nil {
				t.Fatal(e)
			}
			var body map[string]any
			if e = json.Unmarshal(public, &body); e != nil {
				t.Fatal(e)
			}
			if _, ok := body["rowOrder"]; ok {
				t.Fatal("storage metadata leaked into API")
			}
			if e = db.Check(ctx); e != nil {
				t.Fatal(fmt.Errorf("integrity: %w", e))
			}
		})
	}
}

func TestPersistedXrayShareLogRowsDisplayChronologically(t *testing.T) {
	cfg := config.Load()
	cfg.DatabasePath = filepath.Join(t.TempDir(), "player.db")
	ctx := context.Background()
	db, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = storage.Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	early := importer.LogQueryRow{Date: "2026-10-12", Time: "23:59:59", LineNumber: 1}
	late := importer.LogQueryRow{Date: "2026-10-13", Time: "00:00:00", LineNumber: 2}
	snapshot := XrayShareDetailsResponse{Player: importer.XrayPlayerRiskView{RiskScore: 80, Evidence: []importer.XrayEvidenceView{{Type: "TEST", Rows: []importer.LogQueryRow{late, early}}}, RareOreRows: []importer.LogQueryRow{late, early}}}
	payload, e := json.Marshal(snapshot)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	if _, e = db.ExecContext(ctx, `insert into share_links(kind,token,created_at,expires_at,payload_json) values('xray','test',?,?,?)`, now, now.Add(time.Hour), string(payload)); e != nil {
		t.Fatal(e)
	}
	result, e := NewService(db, cfg, nil, nil, nil, nil).XrayDetails(ctx, "test")
	if e != nil {
		t.Fatal(e)
	}
	if result.Player.RiskScore != 80 || result.Player.Evidence[0].Rows[0].LineNumber != 1 || result.Player.RareOreRows[0].LineNumber != 1 {
		t.Fatal("persisted report logs not chronological")
	}
	var unchanged string
	if e = db.QueryRowContext(ctx, `select payload_json from share_links where token='test'`).Scan(&unchanged); e != nil || unchanged != string(payload) {
		t.Fatal("reading report modified persisted analysis", e)
	}
}
