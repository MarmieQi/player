package groupchat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func queryFixture(t *testing.T, text string) (*QueryService, string, QueryRequest) {
	t.Helper()
	zone := time.FixedZone("UTC+8", 8*3600)
	day := time.Date(2026, 10, 12, 0, 0, 0, 0, zone)
	path := filepath.Join(t.TempDir(), "2026-10-12.txt")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	service := NewQueryService(&CopyService{}, zone)
	t.Cleanup(service.Close)
	service.now = func() time.Time { return day.Add(12 * time.Hour) }
	service.files = func(context.Context, time.Time, time.Time) (queryFiles, error) {
		return queryFiles{files: []queryFile{{path, "2026-10-12.txt", day}}, missing: []string{}, release: func() {}}, nil
	}
	return service, path, QueryRequest{FromDate: "2026-10-12", ToDate: "2026-10-12"}
}

func TestChatSearchWholeFileRawKeywordAndExactPlayer(t *testing.T) {
	text := "\ufeff[2026-10-12 00:00:04] [主服][蕨望得芝士404]<A404Fern> 有谁卖书架\r\n" +
		"[2026-10-12 00:02:00] [主服][野生恶魂]<Player With Spaces> 你去冻洋那边开个门\n" +
		"[2026-10-12 00:02:30] [主服][野生恶魂]<Asparagine1> A404Fern 书架\n" +
		"unstructured RAW-ONLY record\n"
	service, _, request := queryFixture(t, text)
	for _, keyword := range []string{"2026-10-12", "主服", "蕨望得芝士404", "a404fern", "书架"} {
		request.Keyword = keyword
		request.Player = "a404fern"
		result, err := service.Search(context.Background(), request)
		if err != nil || result.Total != 1 || result.Rows[0] != strings.Split(text, "\r\n")[0] {
			t.Fatalf("keyword %s: %#v %v", keyword, result, err)
		}
	}
	request.Player = "Player With Spaces"
	request.Keyword = "冻洋"
	result, err := service.Search(context.Background(), request)
	if err != nil || result.Total != 1 {
		t.Fatalf("space account: %#v %v", result, err)
	}
	request.Player = ""
	request.Keyword = "raw-only"
	result, err = service.Search(context.Background(), request)
	if err != nil || result.Total != 1 || result.Rows[0] != "unstructured RAW-ONLY record" {
		t.Fatalf("raw fallback: %#v %v", result, err)
	}
}

func TestChatFiltersBeforePagingAndPagesStayStable(t *testing.T) {
	var text strings.Builder
	for index := 0; index < 350; index++ {
		fmt.Fprintf(&text, "[2026-10-12 01:00:00] [主服][昵称]<Steve> record-%03d\n", index)
	}
	service, path, request := queryFixture(t, text.String())
	first, err := service.Search(context.Background(), request)
	if err != nil || first.Total != 350 || first.PageSize != 200 || len(first.Rows) != 200 || !strings.Contains(first.Rows[0], "record-000") {
		t.Fatalf("first: %#v %v", first, err)
	}
	if err := os.WriteFile(path, []byte("[2026-10-12 01:00:01] [主服][昵称]<Steve> new\n"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := service.Page(first.SnapshotID, 2, 100)
	if err != nil || second.Total != 350 || !strings.Contains(second.Rows[0], "record-100") {
		t.Fatalf("stable page: %#v %v", second, err)
	}
	if err := os.WriteFile(path, []byte(text.String()), 0600); err != nil {
		t.Fatal(err)
	}
	request.Keyword = "record-000"
	match, err := service.Search(context.Background(), request)
	if err != nil || match.Total != 1 || !strings.Contains(match.Rows[0], "record-000") {
		t.Fatalf("whole file: %#v %v", match, err)
	}
	service.now = func() time.Time { return first.ExpiresAt }
	if _, err := service.Page(first.SnapshotID, 1, 100); err == nil {
		t.Fatal("expired page accepted")
	}
}

func TestChatFiveHundredRowPages(t *testing.T) {
	var text strings.Builder
	for index := 0; index < 601; index++ {
		fmt.Fprintf(&text, "[2026-10-12 01:00:00] [主服][昵称]<Steve> record-%03d\n", index)
	}
	service, _, request := queryFixture(t, text.String())
	result, err := service.Search(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	for number, count := range []int{500, 101} {
		page, err := service.Page(result.SnapshotID, number+1, 500)
		if err != nil || page.Total != 601 || page.PageSize != 500 || len(page.Rows) != count {
			t.Fatalf("page %d: %#v %v", number+1, page, err)
		}
		for index, row := range page.Rows {
			if !strings.HasSuffix(row, fmt.Sprintf("record-%03d", number*500+index)) {
				t.Fatalf("unexpected row on page %d: %s", number+1, row)
			}
		}
	}
	if _, err := service.Page(result.SnapshotID, 1, 501); err == nil {
		t.Fatal("page size above 500 accepted")
	}
}

func TestChatSortsChronologicallyAcrossFilesAndPreservesEqualTimeOrder(t *testing.T) {
	line := func(day, clock, body string) string {
		return fmt.Sprintf("[%s %s] [主服][昵称]<Steve> %s", day, clock, body)
	}
	early := line("2026-10-12", "00:01:00", "early")
	first := line("2026-10-12", "08:00:00", "same-time-first")
	second := line("2026-10-12", "08:00:00", "same-time-second")
	late := line("2026-10-12", "23:59:59", "late")
	nextEarly := line("2026-10-13", "00:00:00", "next-day-early")
	nextLate := line("2026-10-13", "08:00:00", "next-day-late")
	service, path, request := queryFixture(t, strings.Join([]string{late, first, early, second}, "\n")+"\n")
	nextPath := filepath.Join(filepath.Dir(path), "2026-10-13.txt")
	if err := os.WriteFile(nextPath, []byte(nextLate+"\n"+nextEarly+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	day, _ := time.ParseInLocation("2006-01-02", request.FromDate, service.zone)
	service.now = func() time.Time { return day.AddDate(0, 0, 1).Add(12 * time.Hour) }
	service.files = func(context.Context, time.Time, time.Time) (queryFiles, error) {
		return queryFiles{files: []queryFile{
			{nextPath, "2026-10-13.txt", day.AddDate(0, 0, 1)},
			{path, "2026-10-12.txt", day},
		}, release: func() {}}, nil
	}
	request.ToDate = "2026-10-13"
	result, err := service.Search(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{early, first, second, late, nextEarly, nextLate}
	if strings.Join(result.Rows, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected chronological order: %v", result.Rows)
	}
	// Pagination must slice the globally sorted snapshot, including midnight.
	page, err := service.Page(result.SnapshotID, 3, 2)
	if err != nil || strings.Join(page.Rows, "\n") != strings.Join(want[4:], "\n") {
		t.Fatalf("cross-date page order: %v %v", page.Rows, err)
	}
}

func TestChatValidationCancellationAndRefreshFailure(t *testing.T) {
	service, _, valid := queryFixture(t, "")
	for _, request := range []QueryRequest{{FromDate: "bad", ToDate: valid.ToDate}, {FromDate: valid.FromDate, ToDate: "2026-10-13"}, {FromDate: "2026-09-01", ToDate: valid.ToDate}, {FromDate: valid.FromDate, ToDate: "2026-10-11"}} {
		if _, err := service.Search(context.Background(), request); err == nil {
			t.Fatalf("accepted %#v", request)
		}
	}
	called := false
	service.files = func(ctx context.Context, _, _ time.Time) (queryFiles, error) {
		called = true
		return queryFiles{}, errors.New("SMB unavailable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Search(ctx, valid); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if called {
		t.Fatal("cancelled query fetched files")
	}
	if _, err := service.Search(context.Background(), valid); err == nil {
		t.Fatal("refresh failure returned success")
	}
	if len(service.snapshots) != 0 {
		t.Fatal("failed query published a snapshot")
	}
}

func TestChatCloseCancelsScanReleasesLeaseAndRejectsLatePublication(t *testing.T) {
	service, _, request := queryFixture(t, "existing result\n")
	if _, err := service.Search(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	allowReturn := make(chan struct{})
	released := make(chan struct{})
	service.files = func(ctx context.Context, _, _ time.Time) (queryFiles, error) {
		close(started)
		<-ctx.Done()
		<-allowReturn
		return queryFiles{release: func() { close(released) }}, nil
	}
	done := make(chan error, 1)
	go func() { _, err := service.Search(context.Background(), request); done <- err }()
	<-started
	service.Close()
	service.Close()
	close(allowReturn)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("closed scan returned success")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("query did not stop on close")
	}
	select {
	case <-released:
	default:
		t.Fatal("lease was not released")
	}
	if len(service.snapshots) != 0 || len(service.slots) != 0 {
		t.Fatal("closed query retained results or scan slot")
	}
	if _, err := service.Search(context.Background(), request); err == nil {
		t.Fatal("closed service accepted query")
	}
}

func TestChatCancellationAfterAcquiringFilesDoesNotPublish(t *testing.T) {
	service, _, request := queryFixture(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	released := false
	service.files = func(context.Context, time.Time, time.Time) (queryFiles, error) {
		cancel()
		return queryFiles{release: func() { released = true }}, nil
	}
	if _, err := service.Search(ctx, request); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if !released || len(service.slots) != 0 || len(service.snapshots) != 0 {
		t.Fatal("cancelled query leaked resources or published a result")
	}
}

func TestChatShutdownCancelsConcurrentQueriesAndBusyQueryDoesNotWait(t *testing.T) {
	service, _, request := queryFixture(t, "")
	started, done := make(chan struct{}, 2), make(chan error, 2)
	service.files = func(ctx context.Context, _, _ time.Time) (queryFiles, error) {
		started <- struct{}{}
		<-ctx.Done()
		return queryFiles{}, ctx.Err()
	}
	for range 2 {
		go func() { _, err := service.Search(context.Background(), request); done <- err }()
	}
	<-started
	<-started
	if _, err := service.Search(context.Background(), request); err == nil {
		t.Fatal("third scan bypassed concurrency limit")
	}
	service.Close()
	for range 2 {
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("scan did not stop")
		}
	}
	if len(service.slots) != 0 {
		t.Fatal("scan slots not released")
	}
}

func TestChatResultCacheIsBounded(t *testing.T) {
	service, _, request := queryFixture(t, "test\n")
	var first string
	for index := 0; index < 10; index++ {
		result, err := service.Search(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			first = result.SnapshotID
		}
		now := service.now().Add(time.Second)
		service.now = func() time.Time { return now }
	}
	if len(service.snapshots) != 8 {
		t.Fatalf("cache size %d", len(service.snapshots))
	}
	if _, err := service.Page(first, 1, 100); err == nil {
		t.Fatal("oldest snapshot was not evicted")
	}
}

func BenchmarkChatSearchTenThousandLines(b *testing.B) {
	zone := time.FixedZone("UTC+8", 8*3600)
	day := time.Date(2026, 10, 12, 0, 0, 0, 0, zone)
	path := filepath.Join(b.TempDir(), "2026-10-12.txt")
	content := strings.Repeat("[2026-10-12 12:34:56] [主服][测试昵称]<Steve> 聊天性能验证，原文保持不变\n", 10000)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		b.Fatal(err)
	}
	service := NewQueryService(&CopyService{}, zone)
	defer service.Close()
	service.files = func(context.Context, time.Time, time.Time) (queryFiles, error) {
		return queryFiles{files: []queryFile{{path, "2026-10-12.txt", day}}, release: func() {}}, nil
	}
	request := QueryRequest{FromDate: "2026-10-12", ToDate: "2026-10-12"}
	b.SetBytes(int64(len(content)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := service.Search(context.Background(), request); err != nil {
			b.Fatal(err)
		}
	}
}
