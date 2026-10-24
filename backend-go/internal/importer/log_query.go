package importer

import (
	"bufio"
	"context"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"player-stats-backend-go/internal/apitype"
	"player-stats-backend-go/internal/auth"
	"player-stats-backend-go/internal/catalog/bedrock"
	"player-stats-backend-go/internal/config"
)

const (
	queryTypeCoordinate    = "coordinate"
	queryTypePlayerKeyword = "playerKeyword"
	defaultLogQueryPage    = 100
	maxLogQueryPageSize    = 500
	defaultPublicLogLimit  = 8
	maxPublicLogLimit      = 5000
	defaultPublicLogDays   = 7
	maxPublicLogDays       = 365
	maxPublicLogRadius     = 100
	maxAdminLogRadius      = 200
)

type LogQueryRequest struct {
	RangeMode  string   `json:"rangeMode"`
	Radius     *float64 `json:"radius"`
	ServerID   string   `json:"serverId"`
	FromDate   string   `json:"fromDate"`
	ToDate     string   `json:"toDate"`
	X1         *float64 `json:"x1"`
	Y1         *float64 `json:"y1"`
	Z1         *float64 `json:"z1"`
	X2         *float64 `json:"x2"`
	Y2         *float64 `json:"y2"`
	Z2         *float64 `json:"z2"`
	Dimension  string   `json:"dimension"`
	QueryType  string   `json:"queryType"`
	PlayerName string   `json:"playerName"`
	Keyword    string   `json:"keyword"`
	Action     string   `json:"action"`
	Actions    []string `json:"actions"`
}

type LogQueryRow struct {
	FileName       string `json:"fileName"`
	FilePath       string `json:"filePath"`
	LineNumber     int64  `json:"lineNumber"`
	Date           string `json:"date"`
	Time           string `json:"time"`
	PlayerName     string `json:"playerName"`
	Action         string `json:"action"`
	X              string `json:"x"`
	Y              string `json:"y"`
	Z              string `json:"z"`
	Dimension      string `json:"dimension"`
	X2             string `json:"x2"`
	Y2             string `json:"y2"`
	Z2             string `json:"z2"`
	Dimension2     string `json:"dimension2"`
	Detail1        string `json:"detail1"`
	Detail1Display string `json:"detail1Display,omitempty"`
	Detail2        string `json:"detail2"`
	Detail2Display string `json:"detail2Display,omitempty"`
}

type LogQueryView struct {
	RangeMode     string        `json:"rangeMode,omitempty"`
	Radius        *float64      `json:"radius,omitempty"`
	JobID         string        `json:"jobId"`
	ServerID      string        `json:"serverId"`
	ServerName    string        `json:"serverName"`
	Status        string        `json:"status"`
	StartedAt     *time.Time    `json:"startedAt"`
	FinishedAt    *time.Time    `json:"finishedAt"`
	FromDate      *apitype.Date `json:"fromDate"`
	ToDate        *apitype.Date `json:"toDate"`
	X1            *float64      `json:"x1"`
	Y1            *float64      `json:"y1"`
	Z1            *float64      `json:"z1"`
	X2            *float64      `json:"x2"`
	Y2            *float64      `json:"y2"`
	Z2            *float64      `json:"z2"`
	Dimension     string        `json:"dimension"`
	ScannedFiles  int           `json:"scannedFiles"`
	ScannedRows   int64         `json:"scannedRows"`
	MatchedRows   int64         `json:"matchedRows"`
	DisplayedRows int           `json:"displayedRows"`
	Page          int           `json:"page"`
	PageSize      int           `json:"pageSize"`
	TotalPages    int           `json:"totalPages"`
	FailedFiles   int           `json:"failedFiles"`
	CurrentFile   string        `json:"currentFile"`
	Message       string        `json:"message"`
	Rows          []LogQueryRow `json:"rows"`
}

type PublicCoordinateLogQueryRequest struct {
	ServerID   string  `json:"serverId"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Z          float64 `json:"z"`
	Radius     float64 `json:"radius"`
	Limit      int     `json:"limit"`
	Days       int     `json:"days"`
	IncludeAll bool    `json:"-"`
}

type PublicCoordinateLogQueryView struct {
	ServerID      string        `json:"serverId"`
	ServerName    string        `json:"serverName"`
	X             float64       `json:"x"`
	Y             float64       `json:"y"`
	Z             float64       `json:"z"`
	Radius        float64       `json:"radius"`
	Days          int           `json:"days"`
	FromDate      *apitype.Date `json:"fromDate"`
	ToDate        *apitype.Date `json:"toDate"`
	ScannedFiles  int           `json:"scannedFiles"`
	ScannedRows   int64         `json:"scannedRows"`
	MatchedRows   int64         `json:"matchedRows"`
	DisplayedRows int           `json:"displayedRows"`
	FailedFiles   int           `json:"failedFiles"`
	Message       string        `json:"message"`
	Rows          []LogQueryRow `json:"rows"`
}

type LogQueryService struct {
	importer *Service
	mu       sync.Mutex
	latest   map[string]*logQueryState
	running  map[string]bool
}

func NewLogQueryService(importer *Service) *LogQueryService {
	return &LogQueryService{
		importer: importer,
		latest:   map[string]*logQueryState{},
		running:  map[string]bool{},
	}
}

func (s *LogQueryService) Start(ctx context.Context, request LogQueryRequest) (LogQueryView, error) {
	criteria, err := s.validateCriteria(request)
	if err != nil {
		return LogQueryView{}, err
	}
	source, ok := s.importer.sourceByID(ctx, criteria.serverID)
	if !ok {
		return LogQueryView{}, auth.NewHTTPError(404, "找不到服务器："+criteria.serverID)
	}
	key := logQueryStateKey(criteria.serverID, criteria.queryType)
	s.mu.Lock()
	if s.running[key] {
		s.mu.Unlock()
		return LogQueryView{}, auth.NewHTTPError(409, source.Name+" 的日志查询正在运行，请稍后再试")
	}
	s.running[key] = true
	state := newLogQueryState(source.ID, source.Name, criteria)
	s.latest[key] = state
	s.mu.Unlock()

	go s.run(state, sourceFromConfig(source), key)
	return state.view(1, defaultLogQueryPage), nil
}

func (s *LogQueryService) Latest(ctx context.Context, serverID, queryType string, page, pageSize int) (LogQueryView, error) {
	serverID, err := requireLogQueryServer(serverID)
	if err != nil {
		return LogQueryView{}, err
	}
	queryType, err = normalizeLogQueryType(queryType)
	if err != nil {
		return LogQueryView{}, err
	}
	key := logQueryStateKey(serverID, queryType)
	s.mu.Lock()
	state := s.latest[key]
	s.mu.Unlock()
	if state != nil {
		return state.view(page, pageSize), nil
	}
	source, ok := s.importer.sourceByID(ctx, serverID)
	if !ok {
		return LogQueryView{}, auth.NewHTTPError(404, "找不到服务器："+serverID)
	}
	return idleLogQueryView(source.ID, source.Name, pageSize), nil
}

func (s *LogQueryService) Clear(ctx context.Context, serverID, queryType string) (LogQueryView, error) {
	serverID, err := requireLogQueryServer(serverID)
	if err != nil {
		return LogQueryView{}, err
	}
	queryType, err = normalizeLogQueryType(queryType)
	if err != nil {
		return LogQueryView{}, err
	}
	key := logQueryStateKey(serverID, queryType)
	s.mu.Lock()
	if s.running[key] {
		s.mu.Unlock()
		return LogQueryView{}, auth.NewHTTPError(409, "日志查询正在运行，完成后再清空结果")
	}
	delete(s.latest, key)
	s.mu.Unlock()
	return s.Latest(ctx, serverID, queryType, 1, defaultLogQueryPage)
}

func (s *LogQueryService) PublicCoordinate(ctx context.Context, request PublicCoordinateLogQueryRequest) (PublicCoordinateLogQueryView, error) {
	serverID, err := requireLogQueryServer(request.ServerID)
	if err != nil {
		return PublicCoordinateLogQueryView{}, err
	}
	source, ok := s.importer.sourceByID(ctx, serverID)
	if !ok {
		return PublicCoordinateLogQueryView{}, auth.NewHTTPError(404, "找不到服务器："+serverID)
	}
	if !finiteFloat(request.Radius) || request.Radius < 0 || request.Radius > maxPublicLogRadius {
		return PublicCoordinateLogQueryView{}, auth.NewHTTPError(400, "查询半径必须在 0-100 格之间")
	}
	files, err := s.importer.localFiles(source)
	if err != nil {
		return PublicCoordinateLogQueryView{}, err
	}
	liveSnapshot, liveSnapshotErr := s.importer.acquireTodayLogSnapshot(ctx, source)
	if liveSnapshot != nil {
		defer liveSnapshot.Release()
		files = mergeTodayLogSnapshotFiles(files, liveSnapshot.Files, s.importer.cfg.Location)
	}
	criteria := logQueryCriteria{
		queryType: queryTypeCoordinate,
		x1:        request.X,
		y1:        request.Y,
		z1:        request.Z,
		x2:        request.X,
		y2:        request.Y,
		z2:        request.Z,
		radius:    &request.Radius,
	}
	days := normalizePublicLogDays(request.Days)
	fromDate, toDate := publicLogDateRangeFromFiles(files, days, s.importer.cfg.Location)
	result := PublicCoordinateLogQueryView{
		ServerID:   source.ID,
		ServerName: source.Name,
		X:          request.X,
		Y:          request.Y,
		Z:          request.Z,
		Radius:     request.Radius,
		Days:       days,
		FromDate:   apiLogDate(fromDate),
		ToDate:     apiLogDate(toDate),
		Rows:       []LogQueryRow{},
	}
	if liveSnapshotErr != nil {
		result.FailedFiles++
	}
	for _, file := range files {
		fileDate := extractLogDate(file.FileName, s.importer.cfg.Location)
		if fromDate == nil || toDate == nil || fileDate == nil || fileDate.Before(*fromDate) || fileDate.After(*toDate) {
			continue
		}
		result.ScannedFiles++
		if err := scanPublicCoordinateFile(file, criteria, &result); err != nil {
			result.FailedFiles++
		}
	}
	SortLogQueryRowsChronologically(result.Rows)
	result.Rows = publicLogRowsForRequest(result.Rows, request)
	result.DisplayedRows = len(result.Rows)
	switch {
	case result.ScannedFiles == 0:
		result.Message = "没有可查询的日志文件"
	case result.MatchedRows == 0:
		result.Message = "没有匹配到这个交互坐标的日志"
	default:
		result.Message = "匹配到 " + strconvI64(result.MatchedRows) + " 条日志，显示最近 " + strconvI64(int64(result.DisplayedRows)) + " 条"
	}
	return result, nil
}

func publicLogRowsForRequest(rows []LogQueryRow, request PublicCoordinateLogQueryRequest) []LogQueryRow {
	if request.IncludeAll {
		return rows
	}
	limit := normalizePublicLogLimit(request.Limit)
	if len(rows) > limit {
		return rows[len(rows)-limit:]
	}
	return rows
}

func scanPublicCoordinateFile(file RemoteLogFile, criteria logQueryCriteria, result *PublicCoordinateLogQueryView) error {
	opened, err := os.Open(file.Path)
	if err != nil {
		return err
	}
	defer opened.Close()
	scanner := bufio.NewScanner(opened)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var lineNumber int64
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		columns := splitPrefix(line)
		if isHeader(columns) {
			continue
		}
		result.ScannedRows++
		if criteria.matches(columns) && isPublicCoordinateAction(valueAt(columns, 3)) {
			result.MatchedRows++
			result.Rows = append(result.Rows, logQueryRow(file, lineNumber, columns))
		}
	}
	return scanner.Err()
}

func isPublicCoordinateAction(raw string) bool {
	if parseAction(raw) != "" {
		return true
	}
	switch strings.TrimSpace(raw) {
	case "放入容器", "从容器取出":
		return true
	}
	return isBucketAction(raw)
}

func (s *LogQueryService) run(state *logQueryState, source configlessSource, key string) {
	state.markRunning()
	defer func() {
		s.mu.Lock()
		state.mu.Lock()
		cancelledQuery(state.task, &state.status, &state.message, &state.finishedAt, &state.currentFile)
		state.mu.Unlock()
		state.task.cancel()
		s.running[key] = false
		s.mu.Unlock()
	}()
	ctx := state.task.ctx
	if ctx.Err() != nil {
		return
	}
	files, err := s.importer.localFiles(source.toConfigSource())
	if err != nil {
		state.markFailed(err.Error())
		return
	}
	if state.criteria.includesDate(dateOnly(time.Now().In(s.importer.cfg.Location), s.importer.cfg.Location)) {
		ctx, cancel := context.WithTimeout(ctx, liveLogSnapshotReadTimeout)
		liveSnapshot, snapshotErr := s.importer.acquireTodayLogSnapshot(ctx, source.toConfigSource())
		cancel()
		if liveSnapshot != nil {
			defer liveSnapshot.Release()
		}
		if state.task.ctx.Err() != nil {
			return
		}
		if snapshotErr != nil {
			state.markFileFailed(RemoteLogFile{FileName: "当天 SMB 日志"}, snapshotErr.Error())
		} else {
			files = mergeTodayLogSnapshotFiles(files, liveSnapshot.Files, s.importer.cfg.Location)
		}
	}
	for _, file := range files {
		if ctx.Err() != nil {
			return
		}
		if !state.criteria.shouldScanFile(file.FileName, s.importer.cfg.Location) {
			continue
		}
		state.markFileStarted(file)
		if err := s.scanFile(state, file); err != nil {
			if ctx.Err() != nil {
				return
			}
			state.markFileFailed(file, err.Error())
		}
	}
	state.markFinished()
}

func (s *LogQueryService) scanFile(state *logQueryState, file RemoteLogFile) error {
	opened, err := os.Open(file.Path)
	if err != nil {
		return err
	}
	defer opened.Close()
	scanner := bufio.NewScanner(opened)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var lineNumber int64
	for scanner.Scan() {
		if err := state.task.ctx.Err(); err != nil {
			return err
		}
		lineNumber++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		columns := splitPrefix(line)
		if isHeader(columns) {
			continue
		}
		state.addScannedRow()
		if state.criteria.matches(columns) {
			state.addRow(logQueryRow(file, lineNumber, columns))
		}
	}
	return scanner.Err()
}

func (s *LogQueryService) validateCriteria(request LogQueryRequest) (logQueryCriteria, error) {
	serverID, err := requireLogQueryServer(request.ServerID)
	if err != nil {
		return logQueryCriteria{}, err
	}
	queryType, err := normalizeLogQueryType(request.QueryType)
	if err != nil {
		return logQueryCriteria{}, err
	}
	fromDate, err := parseOptionalAPIDate(request.FromDate, s.importer.cfg.Location)
	if err != nil {
		return logQueryCriteria{}, err
	}
	toDate, err := parseOptionalAPIDate(request.ToDate, s.importer.cfg.Location)
	if err != nil {
		return logQueryCriteria{}, err
	}
	if fromDate != nil && toDate != nil && fromDate.After(*toDate) {
		return logQueryCriteria{}, auth.NewHTTPError(400, "开始日期不能晚于结束日期")
	}
	criteria := logQueryCriteria{
		serverID:   serverID,
		queryType:  queryType,
		fromDate:   fromDate,
		toDate:     toDate,
		dimension:  strings.TrimSpace(request.Dimension),
		playerName: strings.TrimSpace(request.PlayerName),
		keyword:    strings.TrimSpace(request.Keyword),
		action:     strings.TrimSpace(request.Action),
	}
	if len(request.Actions) > 32 {
		return logQueryCriteria{}, auth.NewHTTPError(400, "行为选项过多")
	}
	for _, action := range request.Actions {
		if action = strings.TrimSpace(action); action != "" {
			if criteria.actions == nil {
				criteria.actions = make(map[string]struct{}, len(request.Actions))
			}
			criteria.actions[action] = struct{}{}
		}
	}
	if queryType == queryTypePlayerKeyword {
		return criteria, nil
	}
	if fromDate == nil || toDate == nil {
		return logQueryCriteria{}, auth.NewHTTPError(400, "请选择查询开始和结束日期")
	}
	mode := strings.TrimSpace(request.RangeMode)
	if mode == "" {
		mode = "box"
	}
	if mode != "box" && mode != "radius" {
		return logQueryCriteria{}, auth.NewHTTPError(400, "不支持的坐标范围方式")
	}
	coords := []*float64{request.X1, request.Y1, request.Z1}
	labels := []string{"起点 X", "起点 Y", "起点 Z"}
	if mode == "radius" {
		labels = []string{"中心 X", "中心 Y", "中心 Z"}
		if request.Radius == nil || !finiteFloat(*request.Radius) || *request.Radius < 0 || *request.Radius > maxAdminLogRadius {
			return logQueryCriteria{}, auth.NewHTTPError(400, "查询半径必须在 0-200 格之间")
		}
	} else {
		coords = append(coords, request.X2, request.Y2, request.Z2)
		labels = append(labels, "终点 X", "终点 Y", "终点 Z")
	}
	for index, value := range coords {
		if value == nil {
			return logQueryCriteria{}, auth.NewHTTPError(400, labels[index]+" 不能为空")
		}
		if !finiteFloat(*value) {
			return logQueryCriteria{}, auth.NewHTTPError(400, labels[index]+" 必须是有限数值")
		}
	}
	criteria.x1, criteria.y1, criteria.z1 = *request.X1, *request.Y1, *request.Z1
	if mode == "radius" {
		radius := *request.Radius
		criteria.radius = &radius
		criteria.x2, criteria.y2, criteria.z2 = criteria.x1, criteria.y1, criteria.z1
	} else {
		criteria.x2, criteria.y2, criteria.z2 = *request.X2, *request.Y2, *request.Z2
	}
	return criteria, nil
}

type configlessSource struct {
	id        string
	name      string
	directory string
	fileGlob  string
	enabled   bool
}

func sourceFromConfig(source config.Source) configlessSource {
	return configlessSource{id: source.ID, name: source.Name, directory: source.Directory, fileGlob: source.FileGlob, enabled: source.Enabled}
}

func (s configlessSource) toConfigSource() config.Source {
	return config.Source{ID: s.id, Name: s.name, Directory: s.directory, FileGlob: s.fileGlob, Enabled: s.enabled}
}

type logQueryCriteria struct {
	serverID   string
	queryType  string
	fromDate   *time.Time
	toDate     *time.Time
	x1         float64
	y1         float64
	z1         float64
	x2         float64
	y2         float64
	z2         float64
	dimension  string
	playerName string
	keyword    string
	action     string
	actions    map[string]struct{}
	radius     *float64
}

func (c logQueryCriteria) shouldScanFile(fileName string, location *time.Location) bool {
	fileDate := extractLogDate(fileName, location)
	if fileDate == nil {
		return false
	}
	if c.fromDate != nil && fileDate.Before(*c.fromDate) {
		return false
	}
	return c.toDate == nil || !fileDate.After(*c.toDate)
}

func (c logQueryCriteria) includesDate(target time.Time) bool {
	if c.fromDate != nil && target.Before(*c.fromDate) {
		return false
	}
	return c.toDate == nil || !target.After(*c.toDate)
}

func (c logQueryCriteria) matches(columns []string) bool {
	if c.queryType == queryTypePlayerKeyword {
		if len(c.actions) > 0 {
			if _, ok := c.actions[valueAt(columns, 3)]; !ok {
				return false
			}
		} else if c.action != "" && !containsFold(valueAt(columns, 3), c.action) {
			return false
		}
		if c.playerName != "" && !strings.EqualFold(valueAt(columns, 2), c.playerName) {
			return false
		}
		if c.keyword != "" && !containsFold(valueAt(columns, 12), c.keyword) && !containsFold(valueAt(columns, 13), c.keyword) {
			return false
		}
		return true
	}
	x, y, z, dimension := interactionCoordinates(columns)
	return c.pointInBox(x, y, z, dimension)
}

func interactionCoordinates(columns []string) (x, y, z, dimension string) {
	x, y, z, dimension = valueAt(columns, 8), valueAt(columns, 9), valueAt(columns, 10), valueAt(columns, 11)
	if !blankCoordinate(x) || !blankCoordinate(y) || !blankCoordinate(z) {
		return x, y, z, dimension
	}

	// Legacy JS and native PlayerLogger releases stored bucket interaction coordinates in position 1.
	if isBucketAction(valueAt(columns, 3)) {
		return valueAt(columns, 4), valueAt(columns, 5), valueAt(columns, 6), valueAt(columns, 7)
	}
	return x, y, z, dimension
}

func (c logQueryCriteria) pointInBox(rawX, rawY, rawZ, rawDimension string) bool {
	if !finiteFloat(c.x1) || !finiteFloat(c.y1) || !finiteFloat(c.z1) ||
		!finiteFloat(c.x2) || !finiteFloat(c.y2) || !finiteFloat(c.z2) {
		return false
	}
	if blankCoordinate(rawX) || blankCoordinate(rawY) || blankCoordinate(rawZ) {
		return false
	}
	x, ok := parseFloat(rawX)
	if !ok {
		return false
	}
	y, ok := parseFloat(rawY)
	if !ok {
		return false
	}
	z, ok := parseFloat(rawZ)
	if !ok {
		return false
	}
	if !finiteFloat(x) || !finiteFloat(y) || !finiteFloat(z) {
		return false
	}
	if c.radius != nil {
		if !finiteFloat(*c.radius) || *c.radius < 0 {
			return false
		}
		dx, dy, dz := x-c.x1, y-c.y1, z-c.z1
		radiusSquared := *c.radius * *c.radius
		return dx*dx+dy*dy+dz*dz <= radiusSquared &&
			(c.dimension == "" || strings.TrimSpace(rawDimension) == c.dimension)
	}
	if x < min(c.x1, c.x2) || x > max(c.x1, c.x2) {
		return false
	}
	if y < min(c.y1, c.y2) || y > max(c.y1, c.y2) {
		return false
	}
	if z < min(c.z1, c.z2) || z > max(c.z1, c.z2) {
		return false
	}
	return c.dimension == "" || strings.TrimSpace(rawDimension) == c.dimension
}

type logQueryState struct {
	task         *queryTask
	mu           sync.Mutex
	jobID        string
	serverID     string
	serverName   string
	criteria     logQueryCriteria
	startedAt    time.Time
	finishedAt   *time.Time
	status       string
	scannedFiles int
	scannedRows  int64
	failedFiles  int
	currentFile  string
	message      string
	rows         []LogQueryRow
}

func newLogQueryState(serverID, serverName string, criteria logQueryCriteria) *logQueryState {
	return &logQueryState{task: newQueryTask(), jobID: newJobID(), serverID: serverID, serverName: serverName, criteria: criteria, startedAt: time.Now().UTC(), status: "PENDING"}
}

func (s *logQueryState) markRunning() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.task.ctx.Err() == nil {
		s.status = "RUNNING"
		s.message = "正在扫描 CSV"
	}
}

func (s *logQueryState) markFileStarted(file RemoteLogFile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scannedFiles++
	s.currentFile = file.FileName
}

func (s *logQueryState) markFileFailed(file RemoteLogFile, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedFiles++
	s.currentFile = file.FileName
	s.message = "部分文件无法解析：" + message
}

func (s *logQueryState) addScannedRow() {
	s.mu.Lock()
	s.scannedRows++
	s.mu.Unlock()
}

func (s *logQueryState) addRow(row LogQueryRow) {
	s.mu.Lock()
	s.rows = append(s.rows, row)
	s.mu.Unlock()
}

func (s *logQueryState) markFinished() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.task != nil && s.task.ctx.Err() != nil {
		return
	}
	now := time.Now().UTC()
	s.finishedAt = &now
	s.status = "FINISHED"
	if s.failedFiles > 0 {
		s.status = "FINISHED_WITH_ERRORS"
	}
	s.currentFile = ""
	SortLogQueryRowsChronologically(s.rows)
	switch {
	case s.scannedFiles == 0:
		s.message = "没有扫描到日期范围内的 CSV 文件"
	case s.scannedRows == 0:
		s.message = "日期范围内的 CSV 没有可查询事件"
	case len(s.rows) == 0:
		s.message = "没有匹配到符合筛选条件的事件"
	default:
		s.message = "匹配到 " + strconvI64(int64(len(s.rows))) + " 条事件"
	}
}

func (s *logQueryState) markFailed(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.task != nil && s.task.ctx.Err() != nil {
		return
	}
	now := time.Now().UTC()
	s.finishedAt = &now
	s.status = "FAILED"
	s.currentFile = ""
	s.message = message
}

func (s *logQueryState) view(page, pageSize int) LogQueryView {
	s.mu.Lock()
	defer s.mu.Unlock()
	pageSize = normalizeLogPageSize(pageSize)
	totalPages := 0
	if len(s.rows) > 0 {
		totalPages = (len(s.rows) + pageSize - 1) / pageSize
	}
	if page < 1 {
		page = 1
	}
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	start := (page - 1) * pageSize
	if start > len(s.rows) {
		start = len(s.rows)
	}
	end := min(start+pageSize, len(s.rows))
	rows := append([]LogQueryRow(nil), s.rows[start:end]...)
	startedAt := s.startedAt
	rangeMode := ""
	if s.criteria.queryType == queryTypeCoordinate {
		rangeMode = "box"
		if s.criteria.radius != nil {
			rangeMode = "radius"
		}
	}
	return LogQueryView{
		RangeMode:     rangeMode,
		Radius:        s.criteria.radius,
		JobID:         s.jobID,
		ServerID:      s.serverID,
		ServerName:    s.serverName,
		Status:        s.status,
		StartedAt:     &startedAt,
		FinishedAt:    s.finishedAt,
		FromDate:      apiLogDate(s.criteria.fromDate),
		ToDate:        apiLogDate(s.criteria.toDate),
		X1:            ptrFloat(s.criteria.x1, s.criteria.queryType == queryTypeCoordinate),
		Y1:            ptrFloat(s.criteria.y1, s.criteria.queryType == queryTypeCoordinate),
		Z1:            ptrFloat(s.criteria.z1, s.criteria.queryType == queryTypeCoordinate),
		X2:            ptrFloat(s.criteria.x2, s.criteria.queryType == queryTypeCoordinate),
		Y2:            ptrFloat(s.criteria.y2, s.criteria.queryType == queryTypeCoordinate),
		Z2:            ptrFloat(s.criteria.z2, s.criteria.queryType == queryTypeCoordinate),
		Dimension:     s.criteria.dimension,
		ScannedFiles:  s.scannedFiles,
		ScannedRows:   s.scannedRows,
		MatchedRows:   int64(len(s.rows)),
		DisplayedRows: len(rows),
		Page:          page,
		PageSize:      pageSize,
		TotalPages:    totalPages,
		FailedFiles:   s.failedFiles,
		CurrentFile:   s.currentFile,
		Message:       s.message,
		Rows:          rows,
	}
}

func idleLogQueryView(serverID, serverName string, pageSize int) LogQueryView {
	pageSize = normalizeLogPageSize(pageSize)
	return LogQueryView{
		ServerID:    serverID,
		ServerName:  serverName,
		Status:      "IDLE",
		Page:        1,
		PageSize:    pageSize,
		Message:     "还没有查询记录",
		Rows:        []LogQueryRow{},
		TotalPages:  0,
		CurrentFile: "",
	}
}

func logQueryRow(file RemoteLogFile, lineNumber int64, values []string) LogQueryRow {
	row := LogQueryRow{
		FileName:   file.FileName,
		FilePath:   file.Path,
		LineNumber: lineNumber,
		Date:       stripBOM(valueAt(values, 0)),
		Time:       valueAt(values, 1),
		PlayerName: valueAt(values, 2),
		Action:     valueAt(values, 3),
		X:          valueAt(values, 4),
		Y:          valueAt(values, 5),
		Z:          valueAt(values, 6),
		Dimension:  valueAt(values, 7),
		X2:         valueAt(values, 8),
		Y2:         valueAt(values, 9),
		Z2:         valueAt(values, 10),
		Dimension2: valueAt(values, 11),
		Detail1:    valueAt(values, 12),
		Detail2:    valueAt(values, 13),
	}
	populateBucketInteractionCoordinates(&row)
	populateLogQueryRowDisplayNames(&row)
	return row
}

func populateBucketInteractionCoordinates(row *LogQueryRow) {
	if row == nil || !isBucketAction(row.Action) {
		return
	}
	if !blankCoordinate(row.X2) || !blankCoordinate(row.Y2) || !blankCoordinate(row.Z2) {
		return
	}
	row.X2 = row.X
	row.Y2 = row.Y
	row.Z2 = row.Z
	row.Dimension2 = row.Dimension
}

func isBucketAction(action string) bool {
	switch strings.TrimSpace(action) {
	case "使用桶放置", "使用桶收取", "使用桶交互":
		return true
	default:
		return false
	}
}

func populateLogQueryRowDisplayNames(row *LogQueryRow) {
	if row == nil {
		return
	}
	if name, ok := bedrock.Lookup(row.Detail1); ok {
		row.Detail1Display = name
	}
	if name, ok := bedrock.Lookup(row.Detail2); ok {
		row.Detail2Display = name
	}
}

func requireLogQueryServer(serverID string) (string, error) {
	serverID = strings.TrimSpace(serverID)
	if serverID == "" || strings.EqualFold(serverID, "all") || strings.EqualFold(serverID, "total") {
		return "", auth.NewHTTPError(400, "请指定主服或 2服")
	}
	return serverID, nil
}

func normalizeLogQueryType(queryType string) (string, error) {
	queryType = strings.TrimSpace(queryType)
	if queryType == "" || queryType == queryTypeCoordinate {
		return queryTypeCoordinate, nil
	}
	if queryType == queryTypePlayerKeyword {
		return queryTypePlayerKeyword, nil
	}
	return "", auth.NewHTTPError(400, "不支持的日志查询类型："+queryType)
}

func parseOptionalAPIDate(raw string, location *time.Location) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := time.ParseInLocation("2006-01-02", raw, location)
	if err != nil {
		return nil, auth.NewHTTPError(400, "日期格式应为 YYYY-MM-DD")
	}
	return &value, nil
}

func logQueryStateKey(serverID, queryType string) string {
	return queryType + ":" + serverID
}

func valueAt(values []string, index int) string {
	if index >= len(values) {
		return ""
	}
	return values[index]
}

func containsFold(value, keyword string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(keyword))
}

func blankCoordinate(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || value == "-"
}

func parseFloat(raw string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	return value, err == nil
}

func finiteFloat(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func normalizeLogPageSize(pageSize int) int {
	if pageSize <= 0 {
		return defaultLogQueryPage
	}
	if pageSize > maxLogQueryPageSize {
		return maxLogQueryPageSize
	}
	return pageSize
}

func normalizePublicLogLimit(limit int) int {
	if limit <= 0 {
		return defaultPublicLogLimit
	}
	if limit > maxPublicLogLimit {
		return maxPublicLogLimit
	}
	return limit
}

func normalizePublicLogDays(days int) int {
	if days <= 0 {
		return defaultPublicLogDays
	}
	if days > maxPublicLogDays {
		return maxPublicLogDays
	}
	return days
}

func publicLogDateRangeFromFiles(files []RemoteLogFile, days int, location *time.Location) (*time.Time, *time.Time) {
	days = normalizePublicLogDays(days)
	var latest *time.Time
	for _, file := range files {
		fileDate := extractLogDate(file.FileName, location)
		if fileDate == nil {
			continue
		}
		if latest == nil || fileDate.After(*latest) {
			value := dateOnly(*fileDate, location)
			latest = &value
		}
	}
	if latest == nil {
		return nil, nil
	}
	from := latest.AddDate(0, 0, -days+1)
	from = dateOnly(from, location)
	to := dateOnly(*latest, location)
	return &from, &to
}

func apiLogDate(value *time.Time) *apitype.Date {
	if value == nil {
		return nil
	}
	date := apitype.NewDate(*value)
	return &date
}

func ptrFloat(value float64, ok bool) *float64 {
	if !ok {
		return nil
	}
	return &value
}

func strconvI64(value int64) string {
	return strconv.FormatInt(value, 10)
}

func (s *LogQueryService) Cancel(ctx context.Context, serverID, queryType, jobID string) (LogQueryView, error) {
	serverID, err := requireLogQueryServer(serverID)
	if err != nil {
		return LogQueryView{}, err
	}
	queryType, err = normalizeLogQueryType(queryType)
	if err != nil {
		return LogQueryView{}, err
	}
	s.mu.Lock()
	state := s.latest[logQueryStateKey(serverID, queryType)]
	if state == nil {
		s.mu.Unlock()
		return LogQueryView{}, auth.NewHTTPError(404, "查询任务不存在")
	}
	state.mu.Lock()
	err = requestQueryCancellation(state.task, state.jobID, jobID, &state.status, &state.message)
	state.mu.Unlock()
	s.mu.Unlock()
	if err != nil {
		return LogQueryView{}, err
	}
	return state.view(1, defaultLogQueryPage), nil
}
