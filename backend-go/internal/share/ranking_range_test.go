package share

import (
	"testing"
	"time"
)

func TestResolveRankingDateRangeUsesAvailableRangeForTotalRanking(t *testing.T) {
	availableFrom := testRankingDate(2025, time.November, 20)
	availableTo := testRankingDate(2026, time.July, 25)

	from, to := resolveRankingDateRange("total", nil, nil, &availableFrom, &availableTo)

	assertRankingDate(t, from, "2025-11-20")
	assertRankingDate(t, to, "2026-07-25")
}

func TestResolveRankingDateRangeAnchorsActiveRankingToLatestParsedLog(t *testing.T) {
	requestedFrom := testRankingDate(2026, time.July, 20)
	requestedTo := testRankingDate(2026, time.July, 26)
	availableFrom := testRankingDate(2025, time.November, 20)
	availableTo := testRankingDate(2026, time.July, 25)

	from, to := resolveRankingDateRange(
		"active",
		&requestedFrom,
		&requestedTo,
		&availableFrom,
		&availableTo,
	)

	assertRankingDate(t, from, "2026-07-19")
	assertRankingDate(t, to, "2026-07-25")
}

func TestResolveRankingDateRangePreservesConfiguredTotalRange(t *testing.T) {
	requestedFrom := testRankingDate(2026, time.January, 1)
	requestedTo := testRankingDate(2026, time.June, 30)
	availableFrom := testRankingDate(2025, time.November, 20)
	availableTo := testRankingDate(2026, time.July, 25)

	from, to := resolveRankingDateRange(
		"total",
		&requestedFrom,
		&requestedTo,
		&availableFrom,
		&availableTo,
	)

	assertRankingDate(t, from, "2026-01-01")
	assertRankingDate(t, to, "2026-06-30")
}

func TestResolvePersistedRankingDateRangeKeepsCreatedActiveWindow(t *testing.T) {
	requestedFrom := testRankingDate(2026, time.July, 19)
	requestedTo := testRankingDate(2026, time.July, 25)
	availableFrom := testRankingDate(2025, time.November, 20)
	availableTo := testRankingDate(2026, time.July, 26)

	from, to := resolvePersistedRankingDateRange(
		"active",
		&requestedFrom,
		&requestedTo,
		&availableFrom,
		&availableTo,
	)

	assertRankingDate(t, from, "2026-07-19")
	assertRankingDate(t, to, "2026-07-25")
}

func testRankingDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.Local)
}

func assertRankingDate(t *testing.T, value *time.Time, expected string) {
	t.Helper()
	if value == nil {
		t.Fatalf("expected %s, got nil", expected)
	}
	if actual := value.Format("2006-01-02"); actual != expected {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}
