package share

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"player-stats-backend-go/internal/storage"
	"slices"
	"strings"
	"sync"
	"time"

	"player-stats-backend-go/internal/apitype"
	"player-stats-backend-go/internal/auth"
	"player-stats-backend-go/internal/catalog"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/importer"
	"player-stats-backend-go/internal/settings"
	"player-stats-backend-go/internal/stats"
)

type Service struct {
	db       *storage.DB
	cfg      config.Config
	settings *settings.Service
	stats    *stats.Service
	xray     *importer.XrayAnalysisService
	logs     *importer.LogQueryService
	mu       sync.Mutex
	xraySubs map[chan XrayGroupMessageResponse]struct{}
}

func NewService(db *storage.DB, cfg config.Config, settingsService *settings.Service, statsService *stats.Service, xrayService *importer.XrayAnalysisService, logQueryService *importer.LogQueryService) *Service {
	return &Service{
		db:       db,
		cfg:      cfg,
		settings: settingsService,
		stats:    statsService,
		xray:     xrayService,
		logs:     logQueryService,
		xraySubs: map[chan XrayGroupMessageResponse]struct{}{},
	}
}

const xrayGroupMessageTTL = 10 * time.Minute
const logQuerySnapshotPageRows = 200
const defaultLogQueryPageSize = 50
const maxLogQueryPageSize = 200

type ShareTokenResponse struct {
	Token      string        `json:"token"`
	SharePath  string        `json:"sharePath"`
	ExpiresAt  time.Time     `json:"expiresAt"`
	TTLMinutes int           `json:"ttlMinutes"`
	FromDate   *apitype.Date `json:"fromDate,omitempty"`
	ToDate     *apitype.Date `json:"toDate,omitempty"`
}

type LogQueryShareTokenResponse struct {
	ShareTokenResponse
	Result importer.PublicCoordinateLogQueryView `json:"result"`
}

type LogQueryShareDetailsResponse struct {
	importer.PublicCoordinateLogQueryView
	ExpiresAt  time.Time `json:"expiresAt"`
	TTLMinutes int       `json:"ttlMinutes"`
	Page       int       `json:"page"`
	PageSize   int       `json:"pageSize"`
	TotalPages int       `json:"totalPages"`
	TotalRows  int       `json:"totalRows"`
}

type PlayerDetailsResponse struct {
	PlayerName    string                  `json:"playerName"`
	LatestLogDate *apitype.Date           `json:"latestLogDate"`
	ExpiresAt     time.Time               `json:"expiresAt"`
	TTLMinutes    int                     `json:"ttlMinutes"`
	Servers       []ServerDetailsResponse `json:"servers"`
}

type ServerDetailsResponse struct {
	ServerID      string                 `json:"serverId"`
	ServerName    string                 `json:"serverName"`
	PlayerName    string                 `json:"playerName"`
	BrokenCount   int64                  `json:"brokenCount"`
	PlacedCount   int64                  `json:"placedCount"`
	TotalCount    int64                  `json:"totalCount"`
	FirstSeenAt   *apitype.LocalDateTime `json:"firstSeenAt"`
	LatestLogDate *apitype.Date          `json:"latestLogDate"`
	Milestones    []MilestoneItem        `json:"milestones"`
	Ores          []OreItem              `json:"ores"`
	Woods         []CountItem            `json:"woods"`
	Saplings      []CountItem            `json:"saplings"`
}

type MilestoneItem struct {
	Type        string                 `json:"type"`
	Label       string                 `json:"label"`
	FoundText   string                 `json:"foundText"`
	MissingText string                 `json:"missingText"`
	FirstSeenAt *apitype.LocalDateTime `json:"firstSeenAt"`
	Detail      string                 `json:"detail"`
}

type OreItem struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	Count int64  `json:"count"`
}

type CountItem struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	Count int64  `json:"count"`
}

type RankingDetailsResponse struct {
	RankingType string                  `json:"rankingType"`
	Limit       int                     `json:"limit"`
	FromDate    *apitype.Date           `json:"fromDate"`
	ToDate      *apitype.Date           `json:"toDate"`
	ExpiresAt   time.Time               `json:"expiresAt"`
	TTLMinutes  int                     `json:"ttlMinutes"`
	Servers     []RankingServerResponse `json:"servers"`
}

type RankingServerResponse struct {
	ServerID   string                `json:"serverId"`
	ServerName string                `json:"serverName"`
	Players    []stats.PlayerSummary `json:"players"`
}

type XrayGroupSendRequest struct {
	ServerID   string                       `json:"serverId"`
	ServerName string                       `json:"serverName"`
	FromTime   string                       `json:"fromTime"`
	ToTime     string                       `json:"toTime"`
	PlayerName string                       `json:"playerName"`
	TTLMinutes *int                         `json:"ttlMinutes"`
	Player     *importer.XrayPlayerRiskView `json:"player"`
}

type XrayGroupSendResponse struct {
	MessageID  int64     `json:"messageId"`
	SharePath  string    `json:"sharePath"`
	Status     string    `json:"status"`
	ServerID   string    `json:"serverId"`
	ServerName string    `json:"serverName"`
	PlayerName string    `json:"playerName"`
	RiskScore  int       `json:"riskScore"`
	RiskLevel  string    `json:"riskLevel"`
	ExpiresAt  time.Time `json:"expiresAt"`
	TTLMinutes int       `json:"ttlMinutes"`
}

type XrayShareDetailsResponse struct {
	ServerID   string                      `json:"serverId"`
	ServerName string                      `json:"serverName"`
	FromTime   *apitype.LocalDateTime      `json:"fromTime"`
	ToTime     *apitype.LocalDateTime      `json:"toTime"`
	PlayerName string                      `json:"playerName"`
	CreatedAt  time.Time                   `json:"createdAt"`
	ExpiresAt  time.Time                   `json:"expiresAt"`
	TTLMinutes int                         `json:"ttlMinutes"`
	Player     importer.XrayPlayerRiskView `json:"player"`
}

type XrayGroupMessageResponse struct {
	ID                         int64                  `json:"id"`
	SharePath                  string                 `json:"sharePath"`
	ServerID                   string                 `json:"serverId"`
	ServerName                 string                 `json:"serverName"`
	PlayerName                 string                 `json:"playerName"`
	RiskScore                  int                    `json:"riskScore"`
	RiskLevel                  string                 `json:"riskLevel"`
	MiningSessionRareOreBreaks int64                  `json:"miningSessionRareOreBreaks"`
	TrackingEvidenceCount      int                    `json:"trackingEvidenceCount"`
	PeakRareOreWindowCount     int                    `json:"peakRareOreWindowCount"`
	FromTime                   *apitype.LocalDateTime `json:"fromTime"`
	ToTime                     *apitype.LocalDateTime `json:"toTime"`
	ExpiresAt                  time.Time              `json:"expiresAt"`
	TTLMinutes                 int                    `json:"ttlMinutes"`
}

type XrayGroupDeliveryRequest struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"errorMessage"`
}

type XrayGroupDeliveryResponse struct {
	MessageID int64  `json:"messageId"`
	Status    string `json:"status"`
}

func (s *Service) CreatePlayerToken(ctx context.Context, playerName string) (ShareTokenResponse, error) {
	playerName = strings.TrimSpace(playerName)
	if playerName == "" {
		return ShareTokenResponse{}, auth.NewHTTPError(httpStatusBadRequest, "playerName is required")
	}
	playerStats, found, err := s.stats.Player(ctx, "", playerName, nil, nil)
	if err != nil {
		return ShareTokenResponse{}, err
	}
	if !found {
		return ShareTokenResponse{}, auth.NewHTTPError(httpStatusNotFound, "玩家没有统计数据")
	}
	if err := s.deleteExpiredTokens(ctx); err != nil {
		return ShareTokenResponse{}, err
	}

	now := time.Now().UTC()
	ttl := s.settings.ShareTTLMinutes(ctx)
	token, err := s.generateUniqueToken(ctx)
	if err != nil {
		return ShareTokenResponse{}, err
	}
	expiresAt := now.Add(time.Duration(ttl) * time.Minute)
	_, err = s.db.ExecContext(ctx, `
		insert into share_links (kind,token,player_name,created_at,expires_at)
        values ('player',?,?,?,?)
	`, token, playerStats.PlayerName, now, expiresAt)
	if err != nil {
		return ShareTokenResponse{}, err
	}
	return ShareTokenResponse{Token: token, SharePath: "/share/" + token, ExpiresAt: expiresAt, TTLMinutes: ttl}, nil
}

func (s *Service) PlayerDetails(ctx context.Context, token string) (PlayerDetailsResponse, error) {
	tokenRow, err := s.playerToken(ctx, token)
	if err != nil {
		return PlayerDetailsResponse{}, err
	}
	playerStats, found, err := s.stats.Player(ctx, "", tokenRow.playerName, nil, nil)
	if err != nil {
		return PlayerDetailsResponse{}, err
	}
	if !found {
		return PlayerDetailsResponse{}, auth.NewHTTPError(httpStatusNotFound, "玩家没有统计数据")
	}

	servers := make([]ServerDetailsResponse, 0, len(playerStats.Servers))
	for _, serverStats := range playerStats.Servers {
		details, err := s.serverDetails(ctx, serverStats, tokenRow.playerName)
		if err != nil {
			return PlayerDetailsResponse{}, err
		}
		servers = append(servers, details)
	}

	return PlayerDetailsResponse{
		PlayerName:    playerStats.PlayerName,
		LatestLogDate: playerStats.LatestLogDate,
		ExpiresAt:     tokenRow.expiresAt,
		TTLMinutes:    ttlMinutes(tokenRow.createdAt, tokenRow.expiresAt),
		Servers:       servers,
	}, nil
}

// RowOrder belongs to persisted snapshot metadata, not the public API.
type logQuerySnapshotMetadata struct {
	importer.PublicCoordinateLogQueryView
	RowOrder string `json:"rowOrder,omitempty"`
}

func (s *Service) CreateLogQueryToken(ctx context.Context, request importer.PublicCoordinateLogQueryRequest) (LogQueryShareTokenResponse, error) {
	if s.logs == nil {
		return LogQueryShareTokenResponse{}, errors.New("log query service is unavailable")
	}
	request.IncludeAll = true
	result, err := s.logs.PublicCoordinate(ctx, request)
	if err != nil {
		return LogQueryShareTokenResponse{}, err
	}
	sanitizeSharedLogRows(&result)
	if err := s.deleteExpiredTokens(ctx); err != nil {
		return LogQueryShareTokenResponse{}, err
	}
	rows := result.Rows
	result.Rows = []importer.LogQueryRow{}
	result.DisplayedRows = len(rows)
	payload, err := json.Marshal(logQuerySnapshotMetadata{PublicCoordinateLogQueryView: result, RowOrder: "asc"})
	if err != nil {
		return LogQueryShareTokenResponse{}, err
	}
	now := time.Now().UTC()
	ttl := s.settings.LogQueryShareTTLMinutes(ctx)
	token, err := s.generateUniqueToken(ctx)
	if err != nil {
		return LogQueryShareTokenResponse{}, err
	}
	expiresAt := now.Add(time.Duration(ttl) * time.Minute)
	compressedPages := [][]byte{}
	for start := 0; start < len(rows); start += logQuerySnapshotPageRows {
		if err := ctx.Err(); err != nil {
			return LogQueryShareTokenResponse{}, err
		}
		end := min(start+logQuerySnapshotPageRows, len(rows))
		compressed, err := compressLogQueryRows(rows[start:end])
		if err != nil {
			return LogQueryShareTokenResponse{}, err
		}
		compressedPages = append(compressedPages, compressed)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LogQueryShareTokenResponse{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		insert into share_links (kind,token,created_at,expires_at,payload_json)
        values ('log',?,?,?,?)
	`, token, now, expiresAt, string(payload))
	if err != nil {
		return LogQueryShareTokenResponse{}, err
	}
	for start := 0; start < len(rows); start += logQuerySnapshotPageRows {
		end := min(start+logQuerySnapshotPageRows, len(rows))
		compressed := compressedPages[start/logQuerySnapshotPageRows]
		_, err = tx.ExecContext(ctx, `
			insert into share_pages (token, page_number, row_count, payload_gzip)
			values (?, ?, ?, ?)
		`, token, start/logQuerySnapshotPageRows+1, end-start, compressed)
		if err != nil {
			return LogQueryShareTokenResponse{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return LogQueryShareTokenResponse{}, err
	}
	return LogQueryShareTokenResponse{
		ShareTokenResponse: ShareTokenResponse{
			Token:      token,
			SharePath:  "/share/logs/" + token,
			ExpiresAt:  expiresAt,
			TTLMinutes: ttl,
			FromDate:   result.FromDate,
			ToDate:     result.ToDate,
		},
		Result: result,
	}, nil
}

func (s *Service) LogQueryDetails(ctx context.Context, token string, page, pageSize int) (LogQueryShareDetailsResponse, error) {
	token = strings.TrimSpace(token)
	var payload string
	var createdAt, expiresAt time.Time
	err := s.db.QueryRowContext(ctx, `
		select payload_json, created_at, expires_at
		from v_log_query_shares
		where token = ?
	`, token).Scan(&payload, &createdAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return LogQueryShareDetailsResponse{}, auth.NewHTTPError(httpStatusNotFound, "链接不存在")
	}
	if err != nil {
		return LogQueryShareDetailsResponse{}, err
	}
	if !expiresAt.After(time.Now().UTC()) {
		_, _ = s.db.ExecContext(ctx, `delete from share_links where token = ?`, token)
		return LogQueryShareDetailsResponse{}, auth.NewHTTPError(httpStatusGone, "链接已过期，请重新在群里查询")
	}
	var metadata logQuerySnapshotMetadata
	if err := json.Unmarshal([]byte(payload), &metadata); err != nil {
		return LogQueryShareDetailsResponse{}, err
	}
	result := metadata.PublicCoordinateLogQueryView
	descendingSnapshot := metadata.RowOrder != "asc"
	pageSize = normalizeLogQueryPageSize(pageSize)
	var snapshotPages, snapshotRows int
	if err := s.db.QueryRowContext(ctx, `
		select count(*), coalesce(sum(row_count), 0)
		from share_pages where token = ?
	`, token).Scan(&snapshotPages, &snapshotRows); err != nil {
		return LogQueryShareDetailsResponse{}, err
	}
	totalRows := result.DisplayedRows
	var pageRows []importer.LogQueryRow
	if snapshotPages > 0 {
		totalRows = snapshotRows
		totalPages := max(1, (totalRows+pageSize-1)/pageSize)
		page = max(1, min(page, totalPages))
		start := (page - 1) * pageSize
		end := min(start+pageSize, totalRows)
		if descendingSnapshot {
			start, end = totalRows-end, totalRows-start
		}
		firstSnapshotPage := start/logQuerySnapshotPageRows + 1
		lastSnapshotPage := (max(start, end-1))/logQuerySnapshotPageRows + 1
		rows, err := s.logQuerySnapshotRows(ctx, token, firstSnapshotPage, lastSnapshotPage)
		if err != nil {
			return LogQueryShareDetailsResponse{}, err
		}
		localStart := start - (firstSnapshotPage-1)*logQuerySnapshotPageRows
		localEnd := min(localStart+(end-start), len(rows))
		if localStart > len(rows) || localEnd-localStart != end-start {
			return LogQueryShareDetailsResponse{}, errors.New("log query snapshot is incomplete")
		}
		if localStart < localEnd {
			pageRows = rows[localStart:localEnd]
			if descendingSnapshot {
				slices.Reverse(pageRows)
			}
		}
	} else {
		// Compatibility with links created before paged snapshots were introduced.
		allRows := result.Rows
		importer.SortLogQueryRowsChronologically(allRows)
		totalRows = len(allRows)
		totalPages := max(1, (totalRows+pageSize-1)/pageSize)
		page = max(1, min(page, totalPages))
		start := min((page-1)*pageSize, totalRows)
		pageRows = allRows[start:min(start+pageSize, totalRows)]
	}
	result.Rows = pageRows
	result.DisplayedRows = totalRows
	sanitizeSharedLogRows(&result)
	totalPages := max(1, (totalRows+pageSize-1)/pageSize)
	return LogQueryShareDetailsResponse{
		PublicCoordinateLogQueryView: result,
		ExpiresAt:                    expiresAt,
		TTLMinutes:                   ttlMinutes(createdAt, expiresAt),
		Page:                         page,
		PageSize:                     pageSize,
		TotalPages:                   totalPages,
		TotalRows:                    totalRows,
	}, nil
}

func compressLogQueryRows(rows []importer.LogQueryRow) ([]byte, error) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if err := json.NewEncoder(writer).Encode(rows); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return compressed.Bytes(), nil
}

func decompressLogQueryRows(payload []byte) ([]importer.LogQueryRow, error) {
	reader, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	var rows []importer.LogQueryRow
	if err := json.Unmarshal(decoded, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) logQuerySnapshotRows(ctx context.Context, token string, fromPage, toPage int) ([]importer.LogQueryRow, error) {
	pages, err := s.db.QueryContext(ctx, `
		select payload_gzip from share_pages
		where token = ? and page_number between ? and ?
		order by page_number
	`, token, fromPage, toPage)
	if err != nil {
		return nil, err
	}
	defer pages.Close()
	result := make([]importer.LogQueryRow, 0, (toPage-fromPage+1)*logQuerySnapshotPageRows)
	for pages.Next() {
		var payload []byte
		if err := pages.Scan(&payload); err != nil {
			return nil, err
		}
		rows, err := decompressLogQueryRows(payload)
		if err != nil {
			return nil, err
		}
		result = append(result, rows...)
	}
	return result, pages.Err()
}

func normalizeLogQueryPageSize(pageSize int) int {
	if pageSize <= 0 {
		return defaultLogQueryPageSize
	}
	return min(pageSize, maxLogQueryPageSize)
}

func sanitizeSharedLogRows(result *importer.PublicCoordinateLogQueryView) {
	for index := range result.Rows {
		result.Rows[index].FilePath = ""
	}
}

func (s *Service) CreateRankingToken(ctx context.Context, rankingType string, limit int, from, to *time.Time) (ShareTokenResponse, error) {
	normalizedType, err := normalizeRankingType(rankingType)
	if err != nil {
		return ShareTokenResponse{}, err
	}
	limit = max(1, min(limit, 20))
	if err := s.deleteExpiredTokens(ctx); err != nil {
		return ShareTokenResponse{}, err
	}
	availableFrom, availableTo, err := s.rankingAvailableDateRange(ctx)
	if err != nil {
		return ShareTokenResponse{}, err
	}
	effectiveFrom, effectiveTo := resolveRankingDateRange(
		normalizedType,
		from,
		to,
		availableFrom,
		availableTo,
	)

	now := time.Now().UTC()
	ttl := s.settings.ShareTTLMinutes(ctx)
	token, err := s.generateUniqueToken(ctx)
	if err != nil {
		return ShareTokenResponse{}, err
	}
	expiresAt := now.Add(time.Duration(ttl) * time.Minute)
	_, err = s.db.ExecContext(ctx, `
		insert into share_links (kind,token,ranking_type,limit_count,from_date,to_date,created_at,expires_at)
        values ('ranking',?,?,?,?,?,?,?)
	`, token, normalizedType, limit, dateArg(effectiveFrom), dateArg(effectiveTo), now, expiresAt)
	if err != nil {
		return ShareTokenResponse{}, err
	}
	return ShareTokenResponse{
		Token:      token,
		SharePath:  "/share/ranking/" + token,
		ExpiresAt:  expiresAt,
		TTLMinutes: ttl,
		FromDate:   apiDatePtr(effectiveFrom),
		ToDate:     apiDatePtr(effectiveTo),
	}, nil
}

func (s *Service) RankingDetails(ctx context.Context, token string) (RankingDetailsResponse, error) {
	tokenRow, err := s.rankingToken(ctx, token)
	if err != nil {
		return RankingDetailsResponse{}, err
	}
	availableFrom, availableTo, err := s.rankingAvailableDateRange(ctx)
	if err != nil {
		return RankingDetailsResponse{}, err
	}
	effectiveFrom, effectiveTo := resolvePersistedRankingDateRange(
		tokenRow.rankingType,
		tokenRow.fromDate,
		tokenRow.toDate,
		availableFrom,
		availableTo,
	)
	servers := make([]RankingServerResponse, 0)
	for _, option := range s.stats.ServerOptions() {
		players, err := s.stats.Players(ctx, option.ServerID, effectiveFrom, effectiveTo, "", tokenRow.limit)
		if err != nil {
			return RankingDetailsResponse{}, err
		}
		servers = append(servers, RankingServerResponse{
			ServerID:   option.ServerID,
			ServerName: option.ServerName,
			Players:    players,
		})
	}
	return RankingDetailsResponse{
		RankingType: tokenRow.rankingType,
		Limit:       tokenRow.limit,
		FromDate:    apiDatePtr(effectiveFrom),
		ToDate:      apiDatePtr(effectiveTo),
		ExpiresAt:   tokenRow.expiresAt,
		TTLMinutes:  ttlMinutes(tokenRow.createdAt, tokenRow.expiresAt),
		Servers:     servers,
	}, nil
}

func resolvePersistedRankingDateRange(
	rankingType string,
	requestedFrom, requestedTo, availableFrom, availableTo *time.Time,
) (*time.Time, *time.Time) {
	if rankingType == "active" && requestedFrom != nil && requestedTo != nil {
		return requestedFrom, requestedTo
	}
	return resolveRankingDateRange(rankingType, requestedFrom, requestedTo, availableFrom, availableTo)
}

func (s *Service) rankingAvailableDateRange(ctx context.Context) (*time.Time, *time.Time, error) {
	var earliest, latest sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		select min(log_date), max(log_date)
		from v_import_files
		where log_date is not null
	`).Scan(&earliest, &latest)
	if err != nil {
		return nil, nil, err
	}
	if earliest.Valid && latest.Valid {
		return datePtr(earliest.Time), datePtr(latest.Time), nil
	}

	err = s.db.QueryRowContext(ctx, `
		select min(stat_date), max(stat_date)
		from v_daily_stats
	`).Scan(&earliest, &latest)
	if err != nil {
		return nil, nil, err
	}
	if !earliest.Valid || !latest.Valid {
		return nil, nil, nil
	}
	return datePtr(earliest.Time), datePtr(latest.Time), nil
}

func resolveRankingDateRange(
	rankingType string,
	requestedFrom, requestedTo, availableFrom, availableTo *time.Time,
) (*time.Time, *time.Time) {
	if rankingType == "active" && availableTo != nil {
		days := inclusiveDays(requestedFrom, requestedTo)
		if days <= 0 {
			days = 7
		}
		to := dateOnly(*availableTo)
		from := to.AddDate(0, 0, -(days - 1))
		return &from, &to
	}

	from := requestedFrom
	to := requestedTo
	if from == nil {
		from = availableFrom
	}
	if to == nil {
		to = availableTo
	}
	return from, to
}

func inclusiveDays(from, to *time.Time) int {
	if from == nil || to == nil {
		return 0
	}
	fromYear, fromMonth, fromDay := from.Date()
	toYear, toMonth, toDay := to.Date()
	fromDate := time.Date(fromYear, fromMonth, fromDay, 0, 0, 0, 0, time.UTC)
	toDate := time.Date(toYear, toMonth, toDay, 0, 0, 0, 0, time.UTC)
	if toDate.Before(fromDate) {
		return 0
	}
	return int(toDate.Sub(fromDate).Hours()/24) + 1
}

func datePtr(value time.Time) *time.Time {
	date := dateOnly(value)
	return &date
}

func dateOnly(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

func (s *Service) SendXrayToGroup(ctx context.Context, request XrayGroupSendRequest) (XrayGroupSendResponse, error) {
	snapshot, err := s.xraySnapshot(ctx, request)
	if err != nil {
		return XrayGroupSendResponse{}, err
	}
	if err := s.deleteExpiredTokens(ctx); err != nil {
		return XrayGroupSendResponse{}, err
	}
	now := time.Now().UTC()
	ttl := normalizeXrayTTL(request.TTLMinutes)
	expiresAt := now.Add(time.Duration(ttl) * time.Minute)
	token, err := s.generateUniqueToken(ctx)
	if err != nil {
		return XrayGroupSendResponse{}, err
	}
	sharePath := "/xray-share/" + token
	details := XrayShareDetailsResponse{
		ServerID:   snapshot.serverID,
		ServerName: snapshot.serverName,
		FromTime:   localDateTimePtr(&snapshot.fromTime),
		ToTime:     localDateTimePtr(&snapshot.toTime),
		PlayerName: snapshot.player.PlayerName,
		CreatedAt:  now,
		ExpiresAt:  expiresAt,
		TTLMinutes: ttl,
		Player:     snapshot.player,
	}
	payload, err := json.Marshal(details)
	if err != nil {
		return XrayGroupSendResponse{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return XrayGroupSendResponse{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		insert into share_links (kind,token,server_id,player_name,created_at,expires_at,payload_json)
        values ('xray',?,?,?,?,?,?)
	`, token, snapshot.serverID, snapshot.player.PlayerName, now, expiresAt, string(payload))
	if err != nil {
		return XrayGroupSendResponse{}, err
	}
	messageID := int64(0)
	response := XrayGroupSendResponse{
		MessageID:  messageID,
		SharePath:  sharePath,
		Status:     "PENDING",
		ServerID:   snapshot.serverID,
		ServerName: snapshot.serverName,
		PlayerName: snapshot.player.PlayerName,
		RiskScore:  snapshot.player.RiskScore,
		RiskLevel:  snapshot.player.RiskLevel,
		ExpiresAt:  expiresAt,
		TTLMinutes: ttl,
	}
	message := XrayGroupMessageResponse{
		ID:                         messageID,
		SharePath:                  sharePath,
		ServerID:                   response.ServerID,
		ServerName:                 response.ServerName,
		PlayerName:                 response.PlayerName,
		RiskScore:                  response.RiskScore,
		RiskLevel:                  response.RiskLevel,
		MiningSessionRareOreBreaks: snapshot.player.AnalysisRareOreBreaks,
		TrackingEvidenceCount:      snapshot.player.AnalysisTrackingEvidenceCount,
		PeakRareOreWindowCount:     snapshot.player.AnalysisPeakRareOreWindowCount,
		FromTime:                   localDateTimePtr(&snapshot.fromTime),
		ToTime:                     localDateTimePtr(&snapshot.toTime),
		ExpiresAt:                  expiresAt,
		TTLMinutes:                 ttl,
	}
	messageID, err = enqueueXrayGroupMessage(ctx, tx, message)
	if err != nil {
		return XrayGroupSendResponse{}, err
	}
	if err = tx.Commit(); err != nil {
		return XrayGroupSendResponse{}, err
	}
	message.ID = messageID
	s.publishXrayGroupMessage(message)
	response.MessageID = messageID
	return response, nil
}

func (s *Service) SubscribeXrayGroupMessages(buffer int) (<-chan XrayGroupMessageResponse, func()) {
	if buffer < 1 {
		buffer = 1
	}
	ch := make(chan XrayGroupMessageResponse, buffer)
	s.mu.Lock()
	s.xraySubs[ch] = struct{}{}
	s.mu.Unlock()
	unsubscribe := func() {
		s.mu.Lock()
		if _, ok := s.xraySubs[ch]; ok {
			delete(s.xraySubs, ch)
			close(ch)
		}
		s.mu.Unlock()
	}
	return ch, unsubscribe
}

// enqueueXrayGroupMessage writes through the share transaction. Notifications
// are published only after both the share and durable outbox have committed.
func enqueueXrayGroupMessage(ctx context.Context, tx *storage.Tx, message XrayGroupMessageResponse) (int64, error) {
	now := time.Now().UTC()
	expires := now.Add(xrayGroupMessageTTL)
	if message.ExpiresAt.Before(expires) {
		expires = message.ExpiresAt
	}
	result, e := tx.ExecContext(ctx, `insert into notification_outbox(status,created_at,expires_at,payload_json) values('PENDING',?,?, '{}')`, now, expires)
	if e != nil {
		return 0, e
	}
	id, e := result.LastInsertId()
	if e != nil {
		return 0, e
	}
	message.ID = id
	payload, e := json.Marshal(message)
	if e != nil {
		return 0, e
	}
	_, e = tx.ExecContext(ctx, `update notification_outbox set payload_json=? where id=?`, string(payload), id)
	return id, e
}
func (s *Service) publishXrayGroupMessage(message XrayGroupMessageResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.xraySubs {
		select {
		case ch <- message:
		default:
		}
	}
}
func (s *Service) XrayDetails(ctx context.Context, token string) (XrayShareDetailsResponse, error) {
	token = strings.TrimSpace(token)
	var payload string
	var expiresAt time.Time
	err := s.db.QueryRowContext(ctx, `select payload_json, expires_at from v_xray_shares where token = ?`, token).Scan(&payload, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return XrayShareDetailsResponse{}, auth.NewHTTPError(httpStatusNotFound, "链接不存在")
	}
	if err != nil {
		return XrayShareDetailsResponse{}, err
	}
	if !expiresAt.After(time.Now().UTC()) {
		return XrayShareDetailsResponse{}, auth.NewHTTPError(httpStatusGone, "链接已过期，请重新生成")
	}
	var details XrayShareDetailsResponse
	if err := json.Unmarshal([]byte(payload), &details); err != nil {
		return XrayShareDetailsResponse{}, err
	}
	importer.SortXrayPlayerLogsChronologically(&details.Player)
	return details, nil
}

func (s *Service) PendingXrayGroupMessages(ctx context.Context, limit int) ([]XrayGroupMessageResponse, error) {
	limit = max(1, min(limit, 10))
	rows, err := s.db.QueryContext(ctx, `select payload_json from notification_outbox where status='PENDING' and expires_at>? order by created_at,id limit ?`, time.Now(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []XrayGroupMessageResponse{}
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var item XrayGroupMessageResponse
		if err := json.Unmarshal([]byte(payload), &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) MarkXrayGroupDelivery(ctx context.Context, messageID int64, success bool, errorMessage string) (XrayGroupDeliveryResponse, error) {
	status := "FAILED"
	if success {
		status = "SENT"
	}
	result, err := s.db.ExecContext(ctx, `update notification_outbox set status=?,delivered_at=?,failure_message=? where id=? and status='PENDING' and expires_at>?`, status, time.Now(), errorMessage, messageID, time.Now())
	if err != nil {
		return XrayGroupDeliveryResponse{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return XrayGroupDeliveryResponse{}, err
	}
	if count == 0 {
		return XrayGroupDeliveryResponse{}, auth.NewHTTPError(httpStatusNotFound, "发送任务不存在或已过期")
	}
	return XrayGroupDeliveryResponse{MessageID: messageID, Status: status}, nil
}

type xrayShareSnapshot struct {
	serverID   string
	serverName string
	fromTime   time.Time
	toTime     time.Time
	player     importer.XrayPlayerRiskView
}

func (s *Service) xraySnapshot(ctx context.Context, request XrayGroupSendRequest) (xrayShareSnapshot, error) {
	serverID := strings.TrimSpace(request.ServerID)
	if serverID == "" {
		serverID = "main"
	}
	playerName := strings.TrimSpace(request.PlayerName)
	if playerName == "" && request.Player != nil {
		playerName = strings.TrimSpace(request.Player.PlayerName)
	}
	if playerName == "" {
		return xrayShareSnapshot{}, auth.NewHTTPError(httpStatusBadRequest, "playerName is required")
	}

	fromTime, ok := parseShareLocalDateTime(request.FromTime, s.cfg.Location)
	if !ok {
		fromTime = time.Now().In(s.cfg.Location)
	}
	toTime, ok := parseShareLocalDateTime(request.ToTime, s.cfg.Location)
	if !ok {
		toTime = fromTime
	}
	serverName := strings.TrimSpace(request.ServerName)
	if serverName == "" {
		serverName = s.cfg.SourceName(serverID)
	}

	if request.Player != nil {
		player := *request.Player
		if strings.TrimSpace(player.PlayerName) == "" {
			player.PlayerName = playerName
		}
		return xrayShareSnapshot{
			serverID:   serverID,
			serverName: serverName,
			fromTime:   fromTime,
			toTime:     toTime,
			player:     player,
		}, nil
	}

	if s.xray == nil {
		return xrayShareSnapshot{}, auth.NewHTTPError(httpStatusBadRequest, "矿透分析服务未初始化")
	}
	analysis, err := s.xray.Latest(ctx, serverID)
	if err != nil {
		return xrayShareSnapshot{}, err
	}
	if analysis.Status != "FINISHED" && analysis.Status != "FINISHED_WITH_ERRORS" {
		return xrayShareSnapshot{}, auth.NewHTTPError(httpStatusBadRequest, "请先完成矿透分析后再发送")
	}
	if analysis.FromTime != nil {
		fromTime = analysis.FromTime.Time
	}
	if analysis.ToTime != nil {
		toTime = analysis.ToTime.Time
	}
	if strings.TrimSpace(analysis.ServerName) != "" {
		serverName = analysis.ServerName
	}
	for _, player := range analysis.Players {
		if strings.EqualFold(player.PlayerName, playerName) {
			return xrayShareSnapshot{
				serverID:   analysis.ServerID,
				serverName: serverName,
				fromTime:   fromTime,
				toTime:     toTime,
				player:     player,
			}, nil
		}
	}
	return xrayShareSnapshot{}, auth.NewHTTPError(httpStatusNotFound, "找不到该玩家的矿透分析结果")
}

func (s *Service) serverDetails(ctx context.Context, server stats.PlayerServerStatsResponse, playerName string) (ServerDetailsResponse, error) {
	milestones, err := s.milestoneItems(ctx, server.ServerID, playerName)
	if err != nil {
		return ServerDetailsResponse{}, err
	}
	milestones = withFirstJoinMilestone(milestones, server.FirstSeenAt)
	ores, err := s.oreItems(ctx, server.ServerID, playerName)
	if err != nil {
		return ServerDetailsResponse{}, err
	}
	woods, err := s.countItems(ctx, server.ServerID, playerName, "v_file_wood", "wood_type", "wood_count", catalog.WoodTypes)
	if err != nil {
		return ServerDetailsResponse{}, err
	}
	saplings, err := s.countItems(ctx, server.ServerID, playerName, "v_file_sapling", "sapling_type", "sapling_count", catalog.SaplingTypes)
	if err != nil {
		return ServerDetailsResponse{}, err
	}
	return ServerDetailsResponse{
		ServerID:      server.ServerID,
		ServerName:    server.ServerName,
		PlayerName:    server.PlayerName,
		BrokenCount:   server.BrokenCount,
		PlacedCount:   server.PlacedCount,
		TotalCount:    server.TotalCount,
		FirstSeenAt:   server.FirstSeenAt,
		LatestLogDate: server.LatestLogDate,
		Milestones:    milestones,
		Ores:          ores,
		Woods:         woods,
		Saplings:      saplings,
	}, nil
}

func withFirstJoinMilestone(items []MilestoneItem, firstSeenAt *apitype.LocalDateTime) []MilestoneItem {
	result := make([]MilestoneItem, 0, len(items)+1)
	result = append(result, MilestoneItem{
		Type:        "FIRST_JOIN",
		Label:       "第一次来到这里",
		FoundText:   "旅程的起点",
		MissingText: "还没有首次记录",
		FirstSeenAt: firstSeenAt,
		Detail:      "旅程的起点",
	})
	return append(result, items...)
}

func (s *Service) milestoneItems(ctx context.Context, serverID, playerName string) ([]MilestoneItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		select m.milestone_type, m.first_seen_at, coalesce(m.detail, '')
		from v_file_milestones m
		join v_import_files f on f.id = m.import_file_id
		where f.server_id = ? and lower(m.player_name) = lower(?)
		order by m.first_seen_at
	`, serverID, playerName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type value struct {
		firstSeenAt apitype.LocalDateTime
		detail      string
	}
	firstByType := map[string]value{}
	for rows.Next() {
		var milestoneType, detail string
		var firstSeen time.Time
		if err := rows.Scan(&milestoneType, &firstSeen, &detail); err != nil {
			return nil, err
		}
		if _, exists := firstByType[milestoneType]; exists {
			continue
		}
		firstByType[milestoneType] = value{firstSeenAt: apitype.NewLocalDateTime(firstSeen), detail: detail}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]MilestoneItem, 0, len(catalog.MilestoneTypes))
	for _, typ := range catalog.MilestoneTypes {
		item := MilestoneItem{
			Type:        typ.Type,
			Label:       typ.Label,
			FoundText:   typ.FoundText,
			MissingText: typ.MissingText,
		}
		if seen, ok := firstByType[typ.Type]; ok {
			value := seen.firstSeenAt
			item.FirstSeenAt = &value
			item.Detail = seen.detail
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *Service) oreItems(ctx context.Context, serverID, playerName string) ([]OreItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		select o.ore_type, coalesce(sum(o.ore_count), 0)
		from v_file_ore o
		join v_import_files f on f.id = o.import_file_id
		where f.server_id = ? and lower(o.player_name) = lower(?)
		group by o.ore_type
	`, serverID, playerName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	countByType := map[string]int64{}
	for rows.Next() {
		var oreType string
		var count int64
		if err := rows.Scan(&oreType, &count); err != nil {
			return nil, err
		}
		countByType[oreType] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]OreItem, 0, len(catalog.OreTypes))
	for _, typ := range catalog.OreTypes {
		result = append(result, OreItem{Type: typ.Type, Label: typ.Label, Count: countByType[typ.Type]})
	}
	return result, nil
}

func (s *Service) countItems(ctx context.Context, serverID, playerName, table, typeColumn, countColumn string, types []catalog.BlockType) ([]CountItem, error) {
	query := `
		select t.` + typeColumn + `, coalesce(sum(t.` + countColumn + `), 0)
		from ` + table + ` t
		join v_import_files f on f.id = t.import_file_id
		where f.server_id = ? and lower(t.player_name) = lower(?)
		group by t.` + typeColumn
	rows, err := s.db.QueryContext(ctx, query, serverID, playerName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	countByType := map[string]int64{}
	for rows.Next() {
		var typ string
		var count int64
		if err := rows.Scan(&typ, &count); err != nil {
			return nil, err
		}
		countByType[typ] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]CountItem, 0, len(types))
	for _, typ := range types {
		result = append(result, CountItem{Type: typ.Type, Label: typ.Label, Count: countByType[typ.Type]})
	}
	return result, nil
}

type playerTokenRow struct {
	playerName string
	createdAt  time.Time
	expiresAt  time.Time
}

func (s *Service) playerToken(ctx context.Context, token string) (playerTokenRow, error) {
	token = strings.TrimSpace(token)
	var row playerTokenRow
	err := s.db.QueryRowContext(ctx, `
		select player_name, created_at, expires_at
		from v_player_shares
		where token = ?
	`, token).Scan(&row.playerName, &row.createdAt, &row.expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return playerTokenRow{}, auth.NewHTTPError(httpStatusNotFound, "链接不存在")
	}
	if err != nil {
		return playerTokenRow{}, err
	}
	if !row.expiresAt.After(time.Now().UTC()) {
		return playerTokenRow{}, auth.NewHTTPError(httpStatusGone, "链接已过期，请重新在群里查询")
	}
	return row, nil
}

type rankingTokenRow struct {
	rankingType string
	limit       int
	fromDate    *time.Time
	toDate      *time.Time
	createdAt   time.Time
	expiresAt   time.Time
}

func (s *Service) rankingToken(ctx context.Context, token string) (rankingTokenRow, error) {
	token = strings.TrimSpace(token)
	var row rankingTokenRow
	var fromDate, toDate sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		select ranking_type, limit_count, from_date, to_date, created_at, expires_at
		from v_ranking_shares
		where token = ?
	`, token).Scan(&row.rankingType, &row.limit, &fromDate, &toDate, &row.createdAt, &row.expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return rankingTokenRow{}, auth.NewHTTPError(httpStatusNotFound, "链接不存在")
	}
	if err != nil {
		return rankingTokenRow{}, err
	}
	if !row.expiresAt.After(time.Now().UTC()) {
		return rankingTokenRow{}, auth.NewHTTPError(httpStatusGone, "链接已过期，请重新在群里查询")
	}
	if fromDate.Valid {
		row.fromDate = &fromDate.Time
	}
	if toDate.Valid {
		row.toDate = &toDate.Time
	}
	return row, nil
}

func (s *Service) deleteExpiredTokens(ctx context.Context) error {
	return s.db.Write(ctx, func(tx *storage.Tx) error {
		if _, e := tx.ExecContext(ctx, `delete from share_links where expires_at<?`, time.Now()); e != nil {
			return e
		}
		_, e := tx.ExecContext(ctx, `delete from notification_outbox where expires_at<?`, time.Now())
		return e
	})
}

func (s *Service) CleanupExpiredTokens(ctx context.Context) error {
	return s.deleteExpiredTokens(ctx)
}

func (s *Service) generateUniqueToken(ctx context.Context) (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		token, err := randomToken(24)
		if err != nil {
			return "", err
		}
		var exists int
		if err := s.db.QueryRowContext(ctx, `select count(*) from share_links where token = ?`, token).Scan(&exists); err != nil {
			return "", err
		}
		if exists == 0 {
			return token, nil
		}
	}
	return "", errors.New("failed to generate share token")
}

func randomToken(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func normalizeRankingType(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		normalized = "total"
	}
	if normalized != "total" && normalized != "active" {
		return "", auth.NewHTTPError(httpStatusBadRequest, "rankingType must be 'total' or 'active'")
	}
	return normalized, nil
}

func ttlMinutes(createdAt, expiresAt time.Time) int {
	minutes := int(expiresAt.Sub(createdAt).Minutes())
	if minutes < 1 {
		return 1
	}
	return minutes
}

func dateArg(value *time.Time) any {
	if value == nil {
		return nil
	}
	return storage.Date(*value)
}

func apiDatePtr(value *time.Time) *apitype.Date {
	if value == nil {
		return nil
	}
	date := apitype.NewDate(*value)
	return &date
}

func localDateTimePtr(value *time.Time) *apitype.LocalDateTime {
	if value == nil {
		return nil
	}
	local := apitype.NewLocalDateTime(*value)
	return &local
}

func normalizeXrayTTL(value *int) int {
	if value == nil {
		return 1440
	}
	ttl := *value
	if ttl < 5 {
		return 5
	}
	if ttl > 10080 {
		return 10080
	}
	return ttl
}

func parseShareLocalDateTime(raw string, location *time.Location) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.In(location), true
	}
	layouts := []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, raw, location); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

const (
	httpStatusBadRequest = 400
	httpStatusNotFound   = 404
	httpStatusGone       = 410
)
