package importer

import (
	"math"
	"testing"
	"time"
)

func TestLogQueryShouldScanFileRequiresParseableDateLikeJava(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, location)
	to := time.Date(2026, 4, 30, 0, 0, 0, 0, location)
	criteria := logQueryCriteria{fromDate: &from, toDate: &to}

	if criteria.shouldScanFile("player_actions_latest.csv", location) {
		t.Fatal("file without parseable log date was scanned; Java skips these files")
	}
	if !criteria.shouldScanFile("player_actions_2026-04-24.csv", location) {
		t.Fatal("file inside date range was not scanned")
	}
	if criteria.shouldScanFile("player_actions_2026-05-01.csv", location) {
		t.Fatal("file outside date range was scanned")
	}
}

func TestLogQueryCoordinateFilterUsesInteractionCoordinates(t *testing.T) {
	criteria := logQueryCriteria{
		queryType: queryTypeCoordinate,
		x1:        0,
		y1:        0,
		z1:        0,
		x2:        20,
		y2:        80,
		z2:        20,
		dimension: "overworld",
	}

	targetInside := []string{
		"2026-04-01", "12:00:00", "Alex", "破坏方块",
		"999", "999", "999", "overworld",
		"10", "20", "10", "overworld",
		"minecraft:stone", "",
	}
	if !criteria.matches(targetInside) {
		t.Fatal("row with interaction coordinate inside the box did not match")
	}

	playerInsideTargetOutside := []string{
		"2026-04-01", "12:00:00", "Alex", "破坏方块",
		"10", "20", "10", "overworld",
		"999", "999", "999", "overworld",
		"minecraft:stone", "",
	}
	if criteria.matches(playerInsideTargetOutside) {
		t.Fatal("row matched by player position; Java filters by interaction coordinate x2/y2/z2")
	}
}

func TestLogQueryCoordinateFilterRejectsNonFiniteCoordinates(t *testing.T) {
	validRow := []string{
		"2026-04-01", "12:00:00", "Alex", "破坏方块",
		"0", "0", "0", "overworld",
		"10", "20", "30", "overworld",
		"minecraft:stone", "",
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		criteria := logQueryCriteria{
			queryType: queryTypeCoordinate,
			x1:        value,
			y1:        20,
			z1:        30,
			x2:        value,
			y2:        20,
			z2:        30,
		}
		if criteria.matches(validRow) {
			t.Fatalf("row matched non-finite query coordinate %v", value)
		}
	}

	criteria := logQueryCriteria{
		queryType: queryTypeCoordinate,
		x1:        10,
		y1:        20,
		z1:        30,
		x2:        10,
		y2:        20,
		z2:        30,
	}
	rowWithNaN := append([]string(nil), validRow...)
	rowWithNaN[8] = "NaN"
	if criteria.matches(rowWithNaN) {
		t.Fatal("row with non-finite stored coordinate matched")
	}
}

func TestPublicCoordinateRadiusUsesThreeDimensionalDistance(t *testing.T) {
	radius := 5.0
	criteria := logQueryCriteria{
		queryType: queryTypeCoordinate,
		x1:        10,
		y1:        20,
		z1:        30,
		x2:        10,
		y2:        20,
		z2:        30,
		radius:    &radius,
	}
	row := []string{
		"2026-04-01", "12:00:00", "Alex", "破坏方块",
		"0", "0", "0", "overworld",
		"13", "24", "30", "overworld",
		"minecraft:stone", "",
	}
	if !criteria.matches(row) {
		t.Fatal("interaction coordinate exactly on radius did not match")
	}
	row[8], row[9], row[10] = "14", "24", "30"
	if criteria.matches(row) {
		t.Fatal("interaction coordinate outside spherical radius matched")
	}
}

func TestLogQueryPlayerKeywordDateIsFileScopedLikeJava(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, location)
	to := time.Date(2026, 4, 30, 0, 0, 0, 0, location)
	criteria := logQueryCriteria{
		queryType:  queryTypePlayerKeyword,
		fromDate:   &from,
		toDate:     &to,
		playerName: "Alex",
		keyword:    "diamond",
		action:     "破坏",
	}
	row := []string{
		"2026-03-01", "12:00:00", "Alex", "破坏方块",
		"0", "0", "0", "overworld",
		"1", "2", "3", "overworld",
		"minecraft:diamond_ore", "",
	}

	if !criteria.matches(row) {
		t.Fatal("player keyword match re-applied row date; Java only filters the selected files by date")
	}
}

func TestNormalizePublicLogDays(t *testing.T) {
	if got := normalizePublicLogDays(0); got != defaultPublicLogDays {
		t.Fatalf("normalizePublicLogDays(0) = %d, want %d", got, defaultPublicLogDays)
	}
	if got := normalizePublicLogDays(30); got != 30 {
		t.Fatalf("normalizePublicLogDays(30) = %d, want 30", got)
	}
	if got := normalizePublicLogDays(maxPublicLogDays + 1); got != maxPublicLogDays {
		t.Fatalf("normalizePublicLogDays(max+1) = %d, want %d", got, maxPublicLogDays)
	}
}

func TestPublicLogDateRangeUsesLatestLocalLogFile(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	files := []RemoteLogFile{
		{FileName: "player_actions_2026-05-20.csv"},
		{FileName: "player_actions_latest.csv"},
		{FileName: "player_actions_2026-05-29.csv"},
		{FileName: "player_actions_2026-05-24.csv.tmp"},
	}

	from, to := publicLogDateRangeFromFiles(files, 7, location)
	if from == nil || to == nil {
		t.Fatal("range is nil, want latest local log based range")
	}
	if got, want := from.Format("2006-01-02"), "2026-05-23"; got != want {
		t.Fatalf("from = %s, want %s", got, want)
	}
	if got, want := to.Format("2006-01-02"), "2026-05-29"; got != want {
		t.Fatalf("to = %s, want %s", got, want)
	}
}
