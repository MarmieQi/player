package importer

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"player-stats-backend-go/internal/storage"
	"regexp"
	"sort"
	"strings"
	"time"

	"player-stats-backend-go/internal/apitype"
	"player-stats-backend-go/internal/auth"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/settings"
)

type Service struct {
	db            *storage.DB
	cfg           config.Config
	settings      *settings.Service
	parser        parser
	liveSnapshots *liveLogSnapshotCache
	archives      archiveGate
}

type ProgressListener interface {
	FileStarted(serverID, serverName string, file RemoteLogFile)
	FileFinished(result FileImportResult)
}

func NewService(db *storage.DB, cfg config.Config, settingsService *settings.Service) *Service {
	return &Service{
		db:            db,
		cfg:           cfg,
		settings:      settingsService,
		parser:        newParser(cfg.Location),
		liveSnapshots: newLiveLogSnapshotCache(cfg.LiveLogSnapshotDirectory, cfg.LiveLogSnapshotTTLSeconds, cfg.Location),
	}
}

func (s *Service) ListLocalImportFiles(ctx context.Context, requestedServerID string) ([]ImportFileStatus, error) {
	skipToday, err := s.settings.SkipToday(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ImportFileStatus, 0)
	for _, source := range s.selectedSources(ctx, requestedServerID) {
		files, err := s.localFiles(source)
		if err != nil {
			result = append(result, ImportFileStatus{
				ServerID:   source.ID,
				ServerName: source.Name,
				RemotePath: source.Directory,
				FileName:   source.Directory,
				Status:     "FAILED",
				Message:    err.Error(),
			})
			continue
		}
		for _, file := range limitFiles(files, s.cfg.MaxFilesPerRun) {
			status, err := s.importFileStatus(ctx, source, file, skipToday)
			if err != nil {
				return nil, err
			}
			result = append(result, status)
		}
	}
	return result, nil
}

func (s *Service) ListRemoteSMBFiles(ctx context.Context, requestedServerID string) ([]ImportFileStatus, error) {
	return s.listRemoteSMBFiles(ctx, requestedServerID)
}

func (s *Service) ImportFromConfiguredSource(ctx context.Context, requestedServerID string, skipToday bool, listener ProgressListener) (ImportRunResult, error) {
	startedAt := time.Now().UTC()
	files := make([]FileImportResult, 0)
	for _, source := range s.selectedSources(ctx, requestedServerID) {
		release, err := s.archives.acquire(ctx, source.ID)
		if err != nil {
			return summarizeRun(startedAt, files), err
		}
		localFiles, err := s.localFiles(source)
		if err != nil {
			release()
			files = appendResult(files, listener, failedResult(source.ID, source.Name, source.Directory, err.Error()))
			continue
		}
		processed := 0
		for _, file := range localFiles {
			status, e := s.importFileStatus(ctx, source, file, skipToday)
			if e != nil {
				release()
				return summarizeRun(startedAt, files), e
			}
			if status.Status == "IMPORTED" || status.Status == "SKIPPED_TODAY" {
				continue
			}
			if s.cfg.MaxFilesPerRun > 0 && processed >= s.cfg.MaxFilesPerRun {
				break
			}
			processed++
			if err := ctx.Err(); err != nil {
				release()
				return summarizeRun(startedAt, files), err
			}
			if listener != nil {
				listener.FileStarted(source.ID, source.Name, file)
			}
			result := s.importOne(ctx, source, file, skipToday)
			files = appendResult(files, listener, result)
		}
		release()
	}
	return summarizeRun(startedAt, files), nil
}

func (s *Service) SyncFilesFromConfiguredSource(ctx context.Context, requestedServerID string, skipToday bool, listener ProgressListener) (ImportRunResult, error) {
	return s.syncFilesFromSMBSource(ctx, requestedServerID, skipToday, listener)
}

func (s *Service) DeleteImportRecord(ctx context.Context, serverID, remotePath string) (DeleteImportRecordResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return DeleteImportRecordResult{}, err
	}
	defer rollbackQuietly(tx)

	deleted, err := s.deleteImportRecordTx(ctx, tx, serverID, remotePath)
	if err != nil {
		return DeleteImportRecordResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return DeleteImportRecordResult{}, err
	}
	return deleted.result, nil
}

func (s *Service) DeleteImportRecords(ctx context.Context, request DeleteImportRecordsRequest) []DeleteImportRecordResult {
	return s.deleteImportRecords(ctx, request, nil)
}

func (s *Service) deleteImportRecords(ctx context.Context, request DeleteImportRecordsRequest, progress func(int)) []DeleteImportRecordResult {
	if len(request.Files) == 0 {
		return []DeleteImportRecordResult{{Deleted: false, Message: "files are required"}}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return failedDeleteRequest(request, err)
	}
	defer rollbackQuietly(tx)

	results := make([]DeleteImportRecordResult, 0, len(request.Files))
	for index, file := range request.Files {
		if progress != nil {
			progress(index)
		}
		if _, err := tx.ExecContext(ctx, `savepoint delete_file`); err != nil {
			return failedDeleteRequest(request, err)
		}
		deleted, err := s.deleteImportRecordTx(ctx, tx, file.ServerID, file.RemotePath)
		if err != nil {
			if _, e := tx.ExecContext(ctx, `rollback to delete_file`); e != nil {
				return failedDeleteRequest(request, e)
			}
			if _, e := tx.ExecContext(ctx, `release delete_file`); e != nil {
				return failedDeleteRequest(request, e)
			}
			results = append(results, DeleteImportRecordResult{ServerID: file.ServerID, RemotePath: file.RemotePath, Deleted: false, Message: err.Error()})
			continue
		}
		if _, e := tx.ExecContext(ctx, `release delete_file`); e != nil {
			return failedDeleteRequest(request, e)
		}
		results = append(results, deleted.result)
	}
	if progress != nil {
		progress(len(request.Files))
	}
	if err := tx.Commit(); err != nil {
		for i := range results {
			results[i].Deleted = false
			results[i].Message = "事务回滚：" + err.Error()
		}
		return results
	}
	return results
}

func failedDeleteRequest(request DeleteImportRecordsRequest, err error) []DeleteImportRecordResult {
	results := make([]DeleteImportRecordResult, 0, len(request.Files))
	for _, file := range request.Files {
		results = append(results, DeleteImportRecordResult{ServerID: file.ServerID, RemotePath: file.RemotePath, Message: "事务回滚：" + err.Error()})
	}
	return results
}

type deletedImportRecord struct {
	result DeleteImportRecordResult
}

func (s *Service) deleteImportRecordTx(ctx context.Context, tx *storage.Tx, serverID, remotePath string) (deletedImportRecord, error) {
	serverID = strings.TrimSpace(serverID)
	remotePath = strings.TrimSpace(remotePath)
	if serverID == "" || remotePath == "" {
		return deletedImportRecord{}, auth.NewHTTPError(400, "serverId and remotePath are required")
	}
	record, found, err := findImportFileTx(ctx, tx, serverID, remotePath)
	if err != nil {
		return deletedImportRecord{}, err
	}
	if !found {
		return deletedImportRecord{}, auth.NewHTTPError(404, "Import record not found")
	}
	if err := deleteFileAndSyncAffectedSummaries(ctx, tx, record.id, record.serverID); err != nil {
		return deletedImportRecord{}, err
	}
	return deletedImportRecord{
		result: DeleteImportRecordResult{ServerID: serverID, RemotePath: remotePath, Deleted: true, Message: "Import record deleted"},
	}, nil
}

func (s *Service) DeleteLocalFile(ctx context.Context, serverID, remotePath string) (DeleteImportRecordResult, error) {
	serverID = strings.TrimSpace(serverID)
	remotePath = strings.TrimSpace(remotePath)
	if serverID == "" || remotePath == "" {
		return DeleteImportRecordResult{}, auth.NewHTTPError(400, "serverId and remotePath are required")
	}
	if strings.EqualFold(serverID, "all") || strings.EqualFold(serverID, "total") {
		return DeleteImportRecordResult{}, auth.NewHTTPError(400, "请指定单个服务器后再删除本地 CSV")
	}
	source, ok := s.sourceByID(ctx, serverID)
	if !ok {
		return DeleteImportRecordResult{}, auth.NewHTTPError(404, "Server not found")
	}
	release, err := s.archives.acquire(ctx, source.ID)
	if err != nil {
		return DeleteImportRecordResult{}, err
	}
	defer release()
	return deleteLocalArchiveFile(source, remotePath)
}

func deleteLocalArchiveFile(source config.Source, remotePath string) (DeleteImportRecordResult, error) {
	serverID := source.ID
	remotePath = strings.TrimSpace(remotePath)
	if remotePath == "" {
		return DeleteImportRecordResult{}, auth.NewHTTPError(400, "文件路径不能为空")
	}
	base, err := filepath.Abs(source.Directory)
	if err != nil {
		return DeleteImportRecordResult{}, err
	}
	target, err := filepath.Abs(remotePath)
	if err != nil {
		return DeleteImportRecordResult{}, err
	}
	rel, err := filepath.Rel(base, target)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) || filepath.Dir(rel) != "." {
		return DeleteImportRecordResult{}, auth.NewHTTPError(400, "只能删除当前同步目录内的文件")
	}
	if !matchesGlob(target, source.FileGlob) {
		return DeleteImportRecordResult{}, auth.NewHTTPError(400, "只能删除匹配当前日志规则的文件")
	}
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return DeleteImportRecordResult{ServerID: serverID, RemotePath: remotePath, Deleted: false, Message: "本地文件不存在"}, nil
	}
	if err != nil {
		return DeleteImportRecordResult{}, err
	}
	if !info.Mode().IsRegular() {
		return DeleteImportRecordResult{}, auth.NewHTTPError(400, "只能删除本地普通文件")
	}
	if err := os.Remove(target); err != nil {
		return DeleteImportRecordResult{}, err
	}
	return DeleteImportRecordResult{ServerID: serverID, RemotePath: remotePath, Deleted: true, Message: "本地文件已删除"}, nil
}

func (s *Service) DeleteLocalFiles(ctx context.Context, request DeleteImportRecordsRequest) []DeleteImportRecordResult {
	if len(request.Files) == 0 {
		return []DeleteImportRecordResult{{Deleted: false, Message: "files are required"}}
	}
	results := make([]DeleteImportRecordResult, 0, len(request.Files))
	for _, file := range request.Files {
		result, err := s.DeleteLocalFile(ctx, file.ServerID, file.RemotePath)
		if err != nil {
			result = DeleteImportRecordResult{ServerID: file.ServerID, RemotePath: file.RemotePath, Deleted: false, Message: err.Error()}
		}
		results = append(results, result)
	}
	return results
}

func (s *Service) LatestAutoTaskLogs(ctx context.Context, limit int) ([]AutoTaskLogView, error) {
	limit = max(1, min(limit, 200))
	rows, err := s.db.QueryContext(ctx, `
		select id, created_at, coalesce(server_id, ''), coalesce(server_name, ''), coalesce(task_type, ''),
		       coalesce(task_label, ''), coalesce(status, ''), coalesce(message, ''), coalesce(file_details, ''),
		       scanned_files, success_files, skipped_files, failed_files
		from task_runs
        where hidden=0
		order by created_at desc
		limit ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]AutoTaskLogView, 0)
	for rows.Next() {
		var item AutoTaskLogView
		if err := rows.Scan(&item.ID, &item.CreatedAt, &item.ServerID, &item.ServerName, &item.TaskType,
			&item.TaskLabel, &item.Status, &item.Message, &item.FileDetails, &item.ScannedFiles,
			&item.SuccessFiles, &item.SkippedFiles, &item.FailedFiles); err != nil {
			return nil, err
		}
		item.FileDetails = normalizeAutoTaskFileDetails(item.FileDetails)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Service) ClearAutoTaskLogs(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `update task_runs set hidden=1 where status<>'RUNNING'`)
	return err
}

func (s *Service) importOne(ctx context.Context, source config.Source, file RemoteLogFile, skipToday bool) FileImportResult {
	logDate := extractLogDate(file.FileName, s.cfg.Location)
	effectiveDate := logDate
	if effectiveDate == nil {
		value := dateOnly(file.LastModified.In(s.cfg.Location), s.cfg.Location)
		effectiveDate = &value
	}
	today := dateOnly(time.Now().In(s.cfg.Location), s.cfg.Location)
	if skipToday && !effectiveDate.Before(today) {
		return skippedResult(source.ID, source.Name, file.Path, "跳过当天或未来的日志文件")
	}
	status, err := s.importFileStatus(ctx, source, file, skipToday)
	if err != nil {
		return failedResult(source.ID, source.Name, file.Path, err.Error())
	}
	if status.Status == "IMPORTED" {
		return skippedResult(source.ID, source.Name, file.Path, status.Message)
	}
	opened, err := os.Open(file.Path)
	if err != nil {
		return failedResult(source.ID, source.Name, file.Path, err.Error())
	}
	defer opened.Close()
	parsed, err := s.parser.parse(opened)
	if err != nil && err != io.EOF {
		return failedResult(source.ID, source.Name, file.Path, err.Error())
	}
	if err := s.applyImport(ctx, source, file, logDate, parsed); err != nil {
		return failedResult(source.ID, source.Name, file.Path, err.Error())
	}
	return importedResult(source.ID, source.Name, file.Path, parsed.rowCount, parsed.ignoredCount)
}

func (s *Service) applyImport(ctx context.Context, source config.Source, file RemoteLogFile, logDate *time.Time, parsed parsedLogFile) error {
	return s.db.Write(ctx, func(tx *storage.Tx) error {
		record, found, e := findImportFileTx(ctx, tx, source.ID, file.Path)
		if e != nil {
			return e
		}
		if found && record.contentHash == parsed.contentHash && record.parserVersion == parserVersion {
			_, e := tx.ExecContext(ctx, `update import_files set file_size=?,last_modified=? where id=?`, file.Size, file.LastModified, record.id)
			return e
		}
		affected := map[string]struct{}{}
		if found {
			affected, e = loadFileSeenPlayers(ctx, tx, record.id)
			if e != nil {
				return e
			}
		}
		for name := range parsed.firstSeenByPlayer {
			affected[name] = struct{}{}
		}
		id, e := upsertImportFile(ctx, tx, record.id, source, file, logDate, parsed)
		if e != nil {
			return e
		}
		if e = deleteFileDetails(ctx, tx, id); e != nil {
			return e
		}
		if e = insertParsedDetails(ctx, tx, id, parsed); e != nil {
			return e
		}
		return s.syncProfiles(ctx, tx, source.ID, source.Name, affected)
	})
}

func (s *Service) importFileStatus(ctx context.Context, source config.Source, file RemoteLogFile, skipToday bool) (ImportFileStatus, error) {
	logDate := extractLogDate(file.FileName, s.cfg.Location)
	effectiveDate := logDate
	if effectiveDate == nil {
		value := dateOnly(file.LastModified.In(s.cfg.Location), s.cfg.Location)
		effectiveDate = &value
	}
	today := dateOnly(time.Now().In(s.cfg.Location), s.cfg.Location)
	var datePtr *apitype.Date
	if logDate != nil {
		value := apitype.NewDate(*logDate)
		datePtr = &value
	}
	record, found, err := s.findImportFile(ctx, source.ID, file.Path)
	if err != nil {
		return ImportFileStatus{}, err
	}
	if skipToday && !effectiveDate.Before(today) {
		return s.statusFromRecord(source, file, datePtr, record, found, "SKIPPED_TODAY", "跳过当天或未来的日志文件"), nil
	}
	if !found {
		return s.statusFromRecord(source, file, datePtr, record, false, "PENDING", "未导入"), nil
	}
	sameMetadata := record.fileSize == file.Size && record.lastModified.UnixMilli() == file.LastModified.UnixMilli()
	hasRows, err := s.hasImportedDetailRows(ctx, record.id)
	if err != nil {
		return ImportFileStatus{}, err
	}
	if sameMetadata && (hasRows || record.rowCount == 0) && record.parserVersion == parserVersion {
		return s.statusFromRecord(source, file, datePtr, record, true, "IMPORTED", "文件大小和修改时间未变化"), nil
	}
	if sameMetadata {
		return s.statusFromRecord(source, file, datePtr, record, true, "NEEDS_IMPORT", "导入记录不完整，可重新解析"), nil
	}
	return s.statusFromRecord(source, file, datePtr, record, true, "CHANGED", "本地 CSV 已变更，可重新解析"), nil
}

func (s *Service) statusFromRecord(source config.Source, file RemoteLogFile, logDate *apitype.Date, record importedFileRecord, found bool, status, message string) ImportFileStatus {
	var importedAt *time.Time
	rowCount := 0
	ignoredCount := 0
	if found {
		importedAt = &record.importedAt
		rowCount = record.rowCount
		ignoredCount = record.ignoredCount
	}
	return ImportFileStatus{
		ServerID:     source.ID,
		ServerName:   source.Name,
		RemotePath:   file.Path,
		LocalPath:    file.Path,
		FileName:     file.FileName,
		FileSize:     file.Size,
		LastModified: file.LastModified,
		LogDate:      logDate,
		Imported:     found,
		ImportedAt:   importedAt,
		RowCount:     rowCount,
		IgnoredCount: ignoredCount,
		Status:       status,
		Message:      message,
	}
}

func (s *Service) selectedSources(ctx context.Context, requestedServerID string) []config.Source {
	requestedServerID = strings.TrimSpace(requestedServerID)
	all := requestedServerID == "" || strings.EqualFold(requestedServerID, "all") || strings.EqualFold(requestedServerID, "total")
	result := make([]config.Source, 0)
	for _, source := range s.cfg.Sources() {
		if !all && source.ID != requestedServerID {
			continue
		}
		if dbSource, ok, err := s.settings.SourceByID(ctx, source.ID); err == nil && ok {
			if !dbSource.Enabled {
				continue
			}
			if strings.TrimSpace(dbSource.SMBFileGlob) != "" {
				source.FileGlob = dbSource.SMBFileGlob
			}
			if strings.TrimSpace(dbSource.SourceName) != "" {
				source.Name = dbSource.SourceName
			}
		}
		result = append(result, source)
	}
	return result
}

func (s *Service) sourceByID(ctx context.Context, serverID string) (config.Source, bool) {
	for _, source := range s.selectedSources(ctx, serverID) {
		if source.ID == serverID {
			return source, true
		}
	}
	return config.Source{}, false
}

func (s *Service) localFiles(source config.Source) ([]RemoteLogFile, error) {
	directory, err := filepath.Abs(source.Directory)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}
	files := make([]RemoteLogFile, 0)
	err = filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if path != directory {
				return filepath.SkipDir
			}
			return nil
		}
		if !matchesGlob(path, source.FileGlob) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil
		}
		files = append(files, RemoteLogFile{
			Path:         filepath.Clean(abs),
			FileName:     info.Name(),
			Size:         info.Size(),
			LastModified: info.ModTime().UTC(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

type importedFileRecord struct {
	id            int64
	serverID      string
	serverName    string
	remotePath    string
	fileName      string
	logDate       sql.NullTime
	fileSize      int64
	lastModified  time.Time
	contentHash   string
	importedAt    time.Time
	rowCount      int
	ignoredCount  int
	parserVersion int
}

var logFileDatePattern = regexp.MustCompile(`.*?(\d{4}-\d{2}-\d{2}).*\.(?:csv|txt)$`)

func extractLogDate(fileName string, location *time.Location) *time.Time {
	match := logFileDatePattern.FindStringSubmatch(fileName)
	if len(match) < 2 {
		return nil
	}
	value, err := time.ParseInLocation("2006-01-02", match[1], location)
	if err != nil {
		return nil
	}
	return &value
}

func nullableDate(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func dateOnly(value time.Time, location *time.Location) time.Time {
	value = value.In(location)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, location)
}

func matchesGlob(path, glob string) bool {
	if strings.TrimSpace(glob) == "" {
		glob = "player_actions_*.csv"
	}
	matched, err := filepath.Match(glob, filepath.Base(path))
	return err == nil && matched
}

func limitFiles(files []RemoteLogFile, limit int) []RemoteLogFile {
	if limit <= 0 {
		limit = 1000
	}
	if len(files) <= limit {
		return files
	}
	return files[:limit]
}

func summarizeRun(startedAt time.Time, files []FileImportResult) ImportRunResult {
	result := ImportRunResult{StartedAt: startedAt, FinishedAt: time.Now().UTC(), ScannedFiles: len(files), Files: files}
	for _, file := range files {
		switch file.Status {
		case "IMPORTED", "COPIED":
			result.ImportedFiles++
		case "SKIPPED":
			result.SkippedFiles++
		case "FAILED":
			result.FailedFiles++
		}
	}
	return result
}

func appendResult(files []FileImportResult, listener ProgressListener, result FileImportResult) []FileImportResult {
	files = append(files, result)
	if listener != nil {
		listener.FileFinished(result)
	}
	return files
}

func rollbackQuietly(tx *storage.Tx) {
	_ = tx.Rollback()
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
