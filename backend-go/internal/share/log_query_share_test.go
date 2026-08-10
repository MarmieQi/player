package share

import (
	"encoding/json"
	"testing"
	"time"

	"player-stats-backend-go/internal/importer"
)

func TestSanitizeSharedLogRowsRemovesLocalPaths(t *testing.T) {
	result := importer.PublicCoordinateLogQueryView{
		Rows: []importer.LogQueryRow{{
			FileName: "player_actions_2026-08-08.csv",
			FilePath: `/data/synced-logs/main/player_actions_2026-08-08.csv`,
		}},
	}

	sanitizeSharedLogRows(&result)

	if result.Rows[0].FilePath != "" {
		t.Fatalf("shared file path = %q, want empty", result.Rows[0].FilePath)
	}
	if result.Rows[0].FileName == "" {
		t.Fatal("shared file name should be preserved")
	}
}

func TestLogQueryShareDetailsJSONIsFlat(t *testing.T) {
	details := LogQueryShareDetailsResponse{
		PublicCoordinateLogQueryView: importer.PublicCoordinateLogQueryView{
			ServerID:   "main",
			ServerName: "主服",
			Rows:       []importer.LogQueryRow{},
		},
		ExpiresAt:  time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC),
		TTLMinutes: 120,
		Page:       2,
		PageSize:   50,
		TotalPages: 6,
		TotalRows:  286,
	}
	payload, err := json.Marshal(details)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["serverId"] != "main" || decoded["serverName"] != "主服" {
		t.Fatalf("unexpected flattened log share payload: %s", payload)
	}
	if decoded["expiresAt"] == nil || decoded["ttlMinutes"] != float64(120) {
		t.Fatalf("missing share metadata: %s", payload)
	}
	if decoded["page"] != float64(2) || decoded["pageSize"] != float64(50) || decoded["totalPages"] != float64(6) || decoded["totalRows"] != float64(286) {
		t.Fatalf("missing pagination metadata: %s", payload)
	}
}

func TestLogQuerySnapshotCompressionRoundTrip(t *testing.T) {
	want := []importer.LogQueryRow{
		{FileName: "player_actions_2026-08-08.csv", LineNumber: 10, PlayerName: "Steve", Action: "破坏方块", X2: "1", Y2: "2", Z2: "3"},
		{FileName: "player_actions_2026-08-08.csv", LineNumber: 11, PlayerName: "Alex", Action: "放置方块", X2: "4", Y2: "5", Z2: "6"},
	}
	payload, err := compressLogQueryRows(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) == 0 {
		t.Fatal("compressed payload is empty")
	}
	got, err := decompressLogQueryRows(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) || got[0].PlayerName != want[0].PlayerName || got[1].Action != want[1].Action {
		t.Fatalf("round trip rows = %#v, want %#v", got, want)
	}
}

func TestNormalizeLogQueryPageSize(t *testing.T) {
	for input, want := range map[int]int{-1: 50, 0: 50, 25: 25, 200: 200, 500: 200} {
		if got := normalizeLogQueryPageSize(input); got != want {
			t.Errorf("normalizeLogQueryPageSize(%d) = %d, want %d", input, got, want)
		}
	}
}
