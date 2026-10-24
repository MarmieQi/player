package importer

import (
	"cmp"
	"slices"
	"strings"
)

func compareLogQueryRows(a, b LogQueryRow) int {
	if order := strings.Compare(a.Date, b.Date); order != 0 {
		return order
	}
	if order := strings.Compare(a.Time, b.Time); order != 0 {
		return order
	}
	if order := strings.Compare(a.FileName, b.FileName); order != 0 {
		return order
	}
	return cmp.Compare(a.LineNumber, b.LineNumber)
}

// SortLogQueryRowsChronologically defines the display order shared by admin
// queries, public log snapshots and mining evidence. Sort before pagination.
func SortLogQueryRowsChronologically(rows []LogQueryRow) {
	if !slices.IsSortedFunc(rows, compareLogQueryRows) {
		slices.SortStableFunc(rows, compareLogQueryRows)
	}
}

// SortXrayPlayerLogsChronologically normalizes a decoded report's log rows;
// player risk ranking and the evidence cards themselves retain their order.
func SortXrayPlayerLogsChronologically(player *XrayPlayerRiskView) {
	for i := range player.Evidence {
		SortLogQueryRowsChronologically(player.Evidence[i].Rows)
	}
	SortLogQueryRowsChronologically(player.RareOreRows)
}
