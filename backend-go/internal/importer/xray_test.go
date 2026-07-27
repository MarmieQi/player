package importer

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestCollectMetricsCountsDiamondAndAncientDebrisLikeJava(t *testing.T) {
	base := time.Date(2026, 5, 23, 7, 45, 0, 0, time.UTC)
	events := []xrayEvent{
		{
			row:        LogQueryRow{Dimension2: "overworld"},
			happenedAt: base,
			oreType:    "DEEPSLATE_DIAMOND_ORE",
			point:      point3{x: 0, y: 20, z: 0},
			hasPoint:   true,
		},
		{
			row:        LogQueryRow{Dimension2: "overworld"},
			happenedAt: base.Add(time.Second),
			oreType:    "DEEPSLATE_DIAMOND_ORE",
			rareOre:    true,
			point:      point3{x: 1, y: -54, z: 0},
			hasPoint:   true,
		},
		{
			row:        LogQueryRow{Dimension2: "nether"},
			happenedAt: base.Add(2 * time.Second),
			oreType:    "ANCIENT_DEBRIS",
			point:      point3{x: 2, y: 40, z: 0},
			hasPoint:   true,
		},
	}

	metrics := collectMetrics(events)
	if metrics.diamondOreBreaks != 2 {
		t.Fatalf("diamondOreBreaks = %d, want 2", metrics.diamondOreBreaks)
	}
	if metrics.ancientDebrisBreaks != 1 {
		t.Fatalf("ancientDebrisBreaks = %d, want 1", metrics.ancientDebrisBreaks)
	}
	if metrics.rareOreBreaks != 3 {
		t.Fatalf("rareOreBreaks = %d, want 3", metrics.rareOreBreaks)
	}
	if metrics.suspiciousRareOreBreaks != 1 {
		t.Fatalf("suspiciousRareOreBreaks = %d, want 1", metrics.suspiciousRareOreBreaks)
	}
	if got := metrics.oreCounts["DEEPSLATE_DIAMOND_ORE"]; got != 2 {
		t.Fatalf("oreCounts[DEEPSLATE_DIAMOND_ORE] = %d, want 2", got)
	}
	if got := metrics.rareOreCounts["DEEPSLATE_DIAMOND_ORE"]; got != 2 {
		t.Fatalf("rareOreCounts[DEEPSLATE_DIAMOND_ORE] = %d, want 2", got)
	}
	if got := metrics.rareOreCounts["ANCIENT_DEBRIS"]; got != 1 {
		t.Fatalf("rareOreCounts[ANCIENT_DEBRIS] = %d, want 1", got)
	}
	if len(metrics.rareRows) != int(metrics.rareOreBreaks) {
		t.Fatalf("rareRows len = %d, rareOreBreaks = %d", len(metrics.rareRows), metrics.rareOreBreaks)
	}
	if metrics.peakRareOreCount != 3 {
		t.Fatalf("peakRareOreCount = %d, want 3", metrics.peakRareOreCount)
	}
	if metrics.suspiciousPeakRareOreCount != 1 {
		t.Fatalf("suspiciousPeakRareOreCount = %d, want 1", metrics.suspiciousPeakRareOreCount)
	}
}

func TestRecentTunnelEventsStopsAtInterruptedPath(t *testing.T) {
	base := time.Date(2026, 5, 23, 7, 45, 0, 0, time.UTC)
	events := []xrayEvent{
		tunnelEvent(base, 0, -54, 0),
		{
			row:        LogQueryRow{Dimension2: "overworld"},
			happenedAt: base.Add(time.Second),
			point:      point3{x: 1, y: -54, z: 0},
			hasPoint:   true,
		},
		tunnelEvent(base.Add(2*time.Second), 2, -54, 0),
	}
	target := xrayEvent{
		row:        LogQueryRow{Dimension2: "overworld"},
		happenedAt: base.Add(3 * time.Second),
		oreType:    "DEEPSLATE_DIAMOND_ORE",
		rareOre:    true,
		point:      point3{x: 3, y: -54, z: 0},
		hasPoint:   true,
	}

	window := recentTunnelEvents(events, target, 180)
	if len(window) != 1 {
		t.Fatalf("recentTunnelEvents len = %d, want 1", len(window))
	}
	if window[0].point.x != 2 {
		t.Fatalf("recentTunnelEvents kept x = %.0f, want 2", window[0].point.x)
	}
}

func TestAttachEvidenceContextRowsUsesTwentyRowsAroundEvidence(t *testing.T) {
	base := time.Date(2026, 5, 23, 7, 45, 0, 0, time.UTC)
	events := make([]xrayEvent, 60)
	for i := range events {
		happenedAt := base.Add(time.Duration(i) * time.Second)
		events[i] = xrayEvent{
			row: LogQueryRow{
				LineNumber: int64(i + 1),
				Date:       happenedAt.Format("2006-01-02"),
				Time:       happenedAt.Format("15:04:05"),
				Action:     "破坏方块",
				Detail1:    "minecraft:deepslate",
			},
			happenedAt: happenedAt,
		}
	}
	evidence := []XrayEvidenceView{
		{
			StartedAt: localDateTimeValue(base.Add(25 * time.Second)),
			EndedAt:   localDateTimeValue(base.Add(26 * time.Second)),
			Summary:   "测试证据",
			Rows:      []LogQueryRow{{LineNumber: 999}},
		},
	}

	enriched := attachEvidenceContextRows(evidence, events, 20)
	if len(enriched) != 1 {
		t.Fatalf("enriched len = %d, want 1", len(enriched))
	}
	if len(enriched[0].Rows) != 42 {
		t.Fatalf("context row len = %d, want 42", len(enriched[0].Rows))
	}
	if got := enriched[0].Rows[0].LineNumber; got != 6 {
		t.Fatalf("first context line = %d, want 6", got)
	}
	if got := enriched[0].Rows[len(enriched[0].Rows)-1].LineNumber; got != 47 {
		t.Fatalf("last context line = %d, want 47", got)
	}
}

func TestXrayDiskStoreRoundTripPreservesEventsAfterWriterEviction(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	store, err := newXrayDiskStore(t.TempDir(), "round-trip", location)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.cleanup(); err != nil {
			t.Error(err)
		}
	})

	first := completeXrayEvent("player-00", time.Date(2026, 7, 1, 12, 30, 45, 123456789, location), 1)
	if err := store.accept(first); err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= xrayDiskWriterLimit; index++ {
		event := completeXrayEvent("player-"+strconvI64(int64(index)), first.happenedAt.Add(time.Duration(index)*time.Second), int64(index+1))
		if err := store.accept(event); err != nil {
			t.Fatal(err)
		}
	}
	second := completeXrayEvent(first.playerName, first.happenedAt.Add(2*time.Hour), 999)
	if err := store.accept(second); err != nil {
		t.Fatal(err)
	}
	if err := store.closeWriters(); err != nil {
		t.Fatal(err)
	}

	events, err := readXrayDiskEvents(store.players[first.playerName], first.playerName, location)
	if err != nil {
		t.Fatal(err)
	}
	want := []xrayEvent{first, second}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("disk events differ after round trip\n got: %#v\nwant: %#v", events, want)
	}
}

func TestXrayDiskStoreResultsMatchMemoryAnalysis(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	store, err := newXrayDiskStore(t.TempDir(), "same-results", location)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.cleanup(); err != nil {
			t.Error(err)
		}
	})

	base := time.Date(2026, 7, 1, 1, 2, 3, 0, location)
	eventsByPlayer := map[string][]xrayEvent{
		"Alpha": {
			ancientDebrisEvent("Alpha", base, 1, 0),
			ancientDebrisEvent("Alpha", base.Add(time.Second), 2, 1),
			ancientDebrisEvent("Alpha", base.Add(2*time.Second), 3, 2),
		},
		"Beta": {
			completeXrayEvent("Beta", base.Add(5*time.Minute), 4),
		},
		"Gamma": {
			ancientDebrisEvent("Gamma", base.Add(10*time.Minute), 5, 0),
			ancientDebrisEvent("Gamma", base.Add(10*time.Minute+time.Second), 6, 1),
			ancientDebrisEvent("Gamma", base.Add(10*time.Minute+2*time.Second), 7, 2),
			ancientDebrisEvent("Gamma", base.Add(10*time.Minute+3*time.Second), 8, 3),
		},
	}
	for _, playerName := range []string{"Gamma", "Alpha", "Beta"} {
		for index := len(eventsByPlayer[playerName]) - 1; index >= 0; index-- {
			if err := store.accept(eventsByPlayer[playerName][index]); err != nil {
				t.Fatal(err)
			}
		}
	}

	got, err := store.results()
	if err != nil {
		t.Fatal(err)
	}
	want := memoryXrayResults(eventsByPlayer)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("disk analysis results differ from memory analysis\n got: %#v\nwant: %#v", got, want)
	}
}

func TestCleanupXrayTempRootOnlyRemovesJobDirectories(t *testing.T) {
	root := t.TempDir()
	staleStore, err := newXrayDiskStore(root, "stale", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	keepDirectory := root + string(filepath.Separator) + "keep"
	if err := os.Mkdir(keepDirectory, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := cleanupXrayTempRoot(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(staleStore.jobDirectory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale job directory still exists or stat failed: %v", err)
	}
	if _, err := os.Stat(keepDirectory); err != nil {
		t.Fatalf("non-job directory should remain: %v", err)
	}
}

func memoryXrayResults(eventsByPlayer map[string][]xrayEvent) []XrayPlayerRiskView {
	results := make([]XrayPlayerRiskView, 0, len(eventsByPlayer))
	for playerName, originalEvents := range eventsByPlayer {
		events := append([]xrayEvent(nil), originalEvents...)
		sort.Slice(events, func(i, j int) bool {
			return events[i].happenedAt.Before(events[j].happenedAt)
		})
		result := analyzePlayer(playerName, events)
		if includeXrayResult(result) {
			results = append(results, result)
		}
	}
	sortXrayResults(results)
	if len(results) > 200 {
		return results[:200]
	}
	return results
}

func completeXrayEvent(playerName string, happenedAt time.Time, lineNumber int64) xrayEvent {
	return xrayEvent{
		row: LogQueryRow{
			FileName:   "player_actions_2026-07-01.csv",
			FilePath:   "/data/synced-logs/main/player_actions_2026-07-01.csv",
			LineNumber: lineNumber,
			Date:       "2026-07-01",
			Time:       "12:30:45",
			PlayerName: playerName,
			Action:     "破坏方块",
			X:          "1.25",
			Y:          "-54",
			Z:          "3.75",
			Dimension:  "overworld",
			X2:         "1.25",
			Y2:         "-54",
			Z2:         "3.75",
			Dimension2: "overworld",
			Detail1:    "minecraft:deepslate_diamond_ore",
			Detail2:    "fortune",
		},
		happenedAt:     happenedAt,
		playerName:     playerName,
		blockID:        "minecraft:deepslate_diamond_ore",
		oreType:        "DEEPSLATE_DIAMOND_ORE",
		rareOre:        true,
		trackingTarget: true,
		tunnelBlock:    false,
		point:          point3{x: 1.25, y: -54, z: 3.75},
		hasPoint:       true,
	}
}

func ancientDebrisEvent(playerName string, happenedAt time.Time, lineNumber int64, x float64) xrayEvent {
	event := completeXrayEvent(playerName, happenedAt, lineNumber)
	event.row.Dimension = "nether"
	event.row.Dimension2 = "nether"
	event.row.X = strconvI64(int64(x))
	event.row.X2 = event.row.X
	event.row.Y = "15"
	event.row.Y2 = "15"
	event.row.Z = "0"
	event.row.Z2 = "0"
	event.row.Detail1 = "minecraft:ancient_debris"
	event.blockID = "minecraft:ancient_debris"
	event.oreType = "ANCIENT_DEBRIS"
	event.point = point3{x: x, y: 15, z: 0}
	return event
}

func tunnelEvent(t time.Time, x, y, z float64) xrayEvent {
	return xrayEvent{
		row:         LogQueryRow{Dimension2: "overworld"},
		happenedAt:  t,
		blockID:     "minecraft:deepslate",
		tunnelBlock: true,
		point:       point3{x: x, y: y, z: z},
		hasPoint:    true,
	}
}
