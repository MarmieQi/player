package groupchat

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"player-stats-backend-go/internal/auth"
)

const (
	queryTTL       = 10 * time.Minute
	maxQueryBytes  = 128 << 20
	maxResultBytes = 16 << 20
	maxCacheBytes  = 64 << 20
	maxQueryRows   = 100000
)

type QueryRequest struct {
	FromDate string `json:"fromDate"`
	ToDate   string `json:"toDate"`
	Player   string `json:"player"`
	Keyword  string `json:"keyword"`
}
type QueryPage struct {
	SnapshotID   string    `json:"snapshotId"`
	CreatedAt    time.Time `json:"createdAt"`
	ExpiresAt    time.Time `json:"expiresAt"`
	Total        int       `json:"total"`
	Page         int       `json:"page"`
	PageSize     int       `json:"pageSize"`
	Rows         []string  `json:"rows"`
	Players      []string  `json:"players"`
	MissingDates []string  `json:"missingDates"`
}
type chatRow struct {
	raw   string
	at    time.Time
	order int
}
type querySnapshot struct {
	id               string
	created, expires time.Time
	rows             []chatRow
	players, missing []string
	bytes            int
}
type QueryService struct {
	zone      *time.Location
	files     func(context.Context, time.Time, time.Time) (queryFiles, error)
	now       func() time.Time
	slots     chan struct{}
	mu        sync.Mutex
	snapshots map[string]*querySnapshot
	lifetime  context.Context
	stop      context.CancelFunc
	closed    bool
}

func NewQueryService(copy *CopyService, zone *time.Location) *QueryService {
	if zone == nil {
		zone = time.Local
	}
	lifetime, stop := context.WithCancel(context.Background())
	return &QueryService{zone: zone, files: copy.queryFiles, now: time.Now, slots: make(chan struct{}, 2), snapshots: map[string]*querySnapshot{}, lifetime: lifetime, stop: stop}
}

var chatPrefix = regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]\s+\[[^\]]*\](?:\[[^\]]*\])?<([^>]+)>`)

func (s *QueryService) validate(r QueryRequest) (time.Time, time.Time, error) {
	from, err := time.ParseInLocation("2006-01-02", r.FromDate, s.zone)
	if err != nil {
		return from, from, auth.NewHTTPError(400, "请选择有效的开始日期")
	}
	to, err := time.ParseInLocation("2006-01-02", r.ToDate, s.zone)
	today, _ := time.ParseInLocation("2006-01-02", s.now().In(s.zone).Format("2006-01-02"), s.zone)
	if err != nil || to.Before(from) || to.After(today) {
		return from, to, auth.NewHTTPError(400, "结束日期须不早于开始日期，且不能晚于今天")
	}
	if to.After(from.AddDate(0, 0, 30)) {
		return from, to, auth.NewHTTPError(400, "一次最多查询 31 天")
	}
	if len(r.Player) > 256 || len(r.Keyword) > 1024 {
		return from, to, auth.NewHTTPError(400, "玩家名或关键词过长")
	}
	return from, to, nil
}

func (s *QueryService) Search(ctx context.Context, request QueryRequest) (QueryPage, error) {
	ctx, cancel := context.WithCancel(ctx)
	stopShutdown := context.AfterFunc(s.lifetime, cancel)
	defer stopShutdown()
	defer cancel()
	if s.lifetime.Err() != nil {
		return QueryPage{}, auth.NewHTTPError(503, "聊天查询服务正在关闭")
	}
	if err := ctx.Err(); err != nil {
		return QueryPage{}, err
	}
	from, to, err := s.validate(request)
	if err != nil {
		return QueryPage{}, err
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return QueryPage{}, ctx.Err()
	default:
		return QueryPage{}, auth.NewHTTPError(429, "聊天查询繁忙，请稍后重试")
	}
	files, err := s.files(ctx, from, to)
	if err != nil {
		return QueryPage{}, err
	}
	defer files.release()
	snapshot, err := scanChatFiles(ctx, files, request, s.zone)
	if err != nil {
		return QueryPage{}, err
	}
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return QueryPage{}, err
	}
	snapshot.id = hex.EncodeToString(token)
	snapshot.created = s.now()
	snapshot.expires = snapshot.created.Add(queryTTL)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return QueryPage{}, auth.NewHTTPError(503, "聊天查询服务正在关闭")
	}
	if err := ctx.Err(); err != nil {
		return QueryPage{}, err
	}
	s.pruneLocked()
	totalBytes := snapshot.bytes
	for _, entry := range s.snapshots {
		totalBytes += entry.bytes
	}
	for len(s.snapshots) >= 8 || totalBytes > maxCacheBytes {
		var oldest *querySnapshot
		for _, entry := range s.snapshots {
			if oldest == nil || entry.created.Before(oldest.created) {
				oldest = entry
			}
		}
		if oldest == nil {
			break
		}
		delete(s.snapshots, oldest.id)
		totalBytes -= oldest.bytes
	}
	s.snapshots[snapshot.id] = snapshot
	return snapshot.page(1, 100), nil
}

func scanChatFiles(ctx context.Context, files queryFiles, request QueryRequest, zone *time.Location) (*querySnapshot, error) {
	result := &querySnapshot{missing: files.missing, rows: []chatRow{}, players: []string{}}
	players := map[string]bool{}
	keyword := strings.ToLower(strings.TrimSpace(request.Keyword))
	player := strings.TrimSpace(request.Player)
	var scanned int64
	for _, entry := range files.files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file, err := os.Open(entry.path)
		if err != nil {
			return nil, fmt.Errorf("读取聊天文件 %s 失败：%w", entry.name, err)
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			file.Close()
			return nil, fmt.Errorf("聊天文件 %s 不可读取", entry.name)
		}
		scanned += info.Size()
		if scanned > maxQueryBytes {
			file.Close()
			return nil, auth.NewHTTPError(413, "查询文件超过 128 MB，请缩小日期范围")
		}
		err = func() error {
			defer file.Close()
			scanner := bufio.NewScanner(io.LimitReader(file, info.Size()))
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)
			for scanner.Scan() {
				if err := ctx.Err(); err != nil {
					return err
				}
				raw := scanner.Text()
				if raw == "" {
					continue
				}
				matched := chatPrefix.FindStringSubmatch(strings.TrimPrefix(raw, "\ufeff"))
				name := ""
				at := entry.day
				if len(matched) > 0 {
					name = matched[2]
					if value, err := time.ParseInLocation("2006-01-02 15:04:05", matched[1], zone); err == nil {
						at = value
					}
					if len(name) <= 256 && len(players) < 10000 && !players[name] {
						// A substring would retain the entire (possibly filtered-out) line.
						name = strings.Clone(name)
						players[name] = true
						result.players = append(result.players, name)
						result.bytes += len(name)
					}
				}
				if player != "" && !strings.EqualFold(name, player) {
					continue
				}
				if keyword != "" && !strings.Contains(strings.ToLower(raw), keyword) {
					continue
				}
				result.bytes += len(raw) + 64
				if result.bytes > maxResultBytes || len(result.rows) >= maxQueryRows {
					return auth.NewHTTPError(413, "匹配结果过多，请增加玩家或关键词条件")
				}
				result.rows = append(result.rows, chatRow{raw, at, len(result.rows)})
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("读取聊天文件 %s 失败：%w", entry.name, err)
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}
	sort.SliceStable(result.rows, func(i, j int) bool {
		if result.rows[i].at.Equal(result.rows[j].at) {
			return result.rows[i].order > result.rows[j].order
		}
		return result.rows[i].at.After(result.rows[j].at)
	})
	sort.Strings(result.players)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *querySnapshot) page(page, size int) QueryPage {
	start := min((page-1)*size, len(s.rows))
	end := min(start+size, len(s.rows))
	rows := make([]string, 0, end-start)
	for _, row := range s.rows[start:end] {
		rows = append(rows, row.raw)
	}
	return QueryPage{s.id, s.created, s.expires, len(s.rows), page, size, rows, s.players, s.missing}
}
func (s *QueryService) Page(id string, page, size int) (QueryPage, error) {
	if page < 1 || page > 100000 || size < 1 || size > 200 {
		return QueryPage{}, auth.NewHTTPError(400, "分页参数无效")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	entry := s.snapshots[id]
	if entry == nil {
		return QueryPage{}, auth.NewHTTPError(410, "查询结果已过期，请重新查询")
	}
	if (page-1)*size >= len(entry.rows) && page != 1 {
		return QueryPage{}, auth.NewHTTPError(400, "页码超出范围")
	}
	return entry.page(page, size), nil
}
func (s *QueryService) pruneLocked() {
	for id, entry := range s.snapshots {
		if !s.now().Before(entry.expires) {
			delete(s.snapshots, id)
		}
	}
}
func (s *QueryService) RunCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	defer s.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			s.pruneLocked()
			s.mu.Unlock()
		}
	}
}

// Close is idempotent. In-flight scans are cancelled and cannot republish results.
func (s *QueryService) Close() {
	s.mu.Lock()
	s.closed = true
	clear(s.snapshots)
	s.mu.Unlock()
	s.stop()
}
