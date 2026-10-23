package importer

import (
	"context"
	"time"

	"player-stats-backend-go/internal/auth"
)

// Task cancellation is separate from deleting retained query results.
type queryTask struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func newQueryTask() *queryTask {
	ctx, cancel := context.WithCancel(context.Background())
	return &queryTask{ctx: ctx, cancel: cancel}
}
func requestQueryCancellation(task *queryTask, currentID, requestedID string, status, message *string) error {
	if requestedID == "" || requestedID != currentID {
		return auth.NewHTTPError(409, "任务已变化，请刷新后重试")
	}
	if *status == "PENDING" || *status == "RUNNING" || *status == "CANCELLING" {
		*status = "CANCELLING"
		*message = "正在取消任务，等待释放资源"
		task.cancel()
	}
	return nil
}
func cancelledQuery(task *queryTask, status, message *string, finishedAt **time.Time, currentFile *string) bool {
	if task == nil || task.ctx.Err() == nil {
		return false
	}
	now := time.Now().UTC()
	*finishedAt = &now
	*status = "CANCELLED"
	*message = "任务已取消，结果未完成"
	*currentFile = ""
	return true
}
