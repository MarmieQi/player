package importer

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"player-stats-backend-go/internal/auth"
)

type DeleteJobView struct {
	JobID          string                     `json:"jobId"`
	Status         string                     `json:"status"`
	TotalFiles     int                        `json:"totalFiles"`
	ProcessedFiles int                        `json:"processedFiles"`
	DeletedFiles   int                        `json:"deletedFiles"`
	FailedFiles    int                        `json:"failedFiles"`
	Message        string                     `json:"message"`
	Results        []DeleteImportRecordResult `json:"results,omitempty"`
}

type deleteJobState struct {
	mu        sync.Mutex
	startedAt time.Time
	viewData  DeleteJobView
}

func (state *deleteJobState) view() DeleteJobView {
	state.mu.Lock()
	defer state.mu.Unlock()
	view := state.viewData
	view.Results = append([]DeleteImportRecordResult(nil), view.Results...)
	return view
}

// Accept promptly; the HTTP request never owns the deletion transaction.
func (s *JobService) StartDeleteRecords(ctx context.Context, request DeleteImportRecordsRequest) (DeleteJobView, error) {
	if err := ctx.Err(); err != nil {
		return DeleteJobView{}, err
	}
	if len(request.Files) == 0 || len(request.Files) > 10000 {
		return DeleteJobView{}, auth.NewHTTPError(400, "请选择 1–10000 个解析记录")
	}
	files := make([]ImportRecordKey, 0, len(request.Files))
	seen := map[string]bool{}
	for _, file := range request.Files {
		file.ServerID, file.RemotePath = strings.TrimSpace(file.ServerID), strings.TrimSpace(file.RemotePath)
		if file.ServerID == "" || file.RemotePath == "" {
			return DeleteJobView{}, auth.NewHTTPError(400, "文件来源和路径不能为空")
		}
		key := fileKey(file.ServerID, filepath.Base(file.RemotePath))
		if !seen[key] {
			files = append(files, file)
			seen[key] = true
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return DeleteJobView{}, auth.NewHTTPError(503, "任务服务正在关闭")
	}
	if s.running {
		return DeleteJobView{}, auth.NewHTTPError(409, "已有解析或删除任务正在运行，请等待完成")
	}
	if s.deleteJobs == nil {
		s.deleteJobs = map[string]*deleteJobState{}
	}
	if len(s.deleteJobs) >= 8 {
		var oldest string
		for id, state := range s.deleteJobs {
			if oldest == "" || state.startedAt.Before(s.deleteJobs[oldest].startedAt) {
				oldest = id
			}
		}
		delete(s.deleteJobs, oldest)
	}
	state := &deleteJobState{startedAt: time.Now(), viewData: DeleteJobView{JobID: newJobID(), Status: "PENDING", TotalFiles: len(files)}}
	s.deleteJobs[state.viewData.JobID] = state
	s.running = true
	go s.runDeleteRecords(state, DeleteImportRecordsRequest{Files: files})
	return state.view(), nil
}

func (s *JobService) GetDeleteJob(id string) (DeleteJobView, error) {
	s.mu.Lock()
	state := s.deleteJobs[id]
	if id == "latest" {
		for _, candidate := range s.deleteJobs {
			if state == nil || candidate.startedAt.After(state.startedAt) {
				state = candidate
			}
		}
	}
	s.mu.Unlock()
	if state == nil {
		return DeleteJobView{}, auth.NewHTTPError(404, "删除任务不存在或服务已重启，请刷新文件列表确认状态")
	}
	return state.view(), nil
}

func (s *JobService) runDeleteRecords(state *deleteJobState, request DeleteImportRecordsRequest) {
	defer func() { s.mu.Lock(); s.running = false; s.mu.Unlock() }()
	ctx, cancel := context.WithTimeout(s.lifetime, 30*time.Minute)
	defer cancel()
	state.mu.Lock()
	state.viewData.Status = "RUNNING"
	state.mu.Unlock()
	results := s.importer.deleteImportRecords(ctx, request, func(processed int) {
		state.mu.Lock()
		state.viewData.ProcessedFiles = processed
		state.mu.Unlock()
	})
	// Report deletion only after the transaction has committed successfully.
	state.mu.Lock()
	defer state.mu.Unlock()
	view := &state.viewData
	view.Results = results
	for _, result := range results {
		if result.Deleted {
			view.DeletedFiles++
		}
	}
	view.FailedFiles = view.TotalFiles - view.DeletedFiles
	view.Status = "FINISHED"
	if view.FailedFiles > 0 {
		view.Status = "FINISHED_WITH_ERRORS"
	}
	if view.DeletedFiles == 0 && view.FailedFiles > 0 {
		view.Status = "FAILED"
		if len(results) > 0 {
			view.Message = "删除失败：" + results[0].Message
		}
	}
	view.ProcessedFiles = view.TotalFiles
	if ctx.Err() != nil && view.DeletedFiles == 0 {
		view.Status = "FAILED"
		view.Message = "删除任务已中断，事务已回滚，请刷新文件列表"
	}
}
