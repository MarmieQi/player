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
}
