package httpapi_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"player-stats-backend-go/internal/importer"
)

func TestAdminLogSortHTTPAcrossEveryPage(t *testing.T) {
	f := newPublicFixture(t)
	var result importer.LogQueryView
	f.request(t, "POST", "/api/import/log-query-jobs", map[string]string{
		"serverId": "main", "queryType": "playerKeyword", "fromDate": "2026-04-01", "toDate": "2026-04-01", "keyword": "minecraft:",
	}, "admin", 202, &result)
	base := "/api/import/log-query-jobs/latest?serverId=main&queryType=playerKeyword"
	deadline := time.Now().Add(5 * time.Second)
	for {
		f.request(t, "GET", base, nil, "admin", 200, &result)
		if result.Status == "FINISHED" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("query did not finish: %+v", result)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if result.PageSize != 200 || result.SortOrder != "asc" || result.MatchedRows != 262 {
		t.Fatalf("bad defaults: %+v", result)
	}
	var ascending []importer.LogQueryRow
	f.request(t, "GET", base+"&pageSize=500", nil, "admin", 200, &result)
	ascending = result.Rows
	for _, size := range []int{50, 200, 500} {
		var got []importer.LogQueryRow
		for page := 1; len(got) < len(ascending); page++ {
			var pageResult importer.LogQueryView
			f.request(t, "GET", base+fmt.Sprintf("&page=%d&pageSize=%d&sortOrder=desc", page, size), nil, "admin", 200, &pageResult)
			if pageResult.SortOrder != "desc" {
				t.Fatal(pageResult.SortOrder)
			}
			got = append(got, pageResult.Rows...)
		}
		for i, row := range got {
			if !reflect.DeepEqual(row, ascending[len(ascending)-1-i]) {
				t.Fatalf("size %d row %d not globally reversed", size, i)
			}
		}
	}
	f.request(t, "GET", base+"&pageSize=500&sortOrder=asc", nil, "admin", 200, &result)
	if !reflect.DeepEqual(result.Rows, ascending) {
		t.Fatal("sorting changed source result")
	}
	f.request(t, "GET", base+"&sortOrder=invalid", nil, "admin", 400, nil)
}
