package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"player-stats-backend-go/internal/auth"
	"player-stats-backend-go/internal/groupchat"
)

func (s *Server) queryGroupChat(w http.ResponseWriter, r *http.Request) {
	var request groupchat.QueryRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&request); err != nil {
		writeError(w, 400, "查询条件无效")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	result, err := s.groupChatQuery.Search(ctx, request)
	if errors.Is(err, context.DeadlineExceeded) {
		err = auth.NewHTTPError(504, "聊天查询超时，请缩小日期范围后重试")
	}
	s.writeHTTPErrorResult(w, 200, result, err)
}
func (s *Server) groupChatQueryPage(w http.ResponseWriter, r *http.Request) {
	page, size := 1, 200
	if value := r.URL.Query().Get("page"); value != "" {
		page, _ = strconv.Atoi(value)
	}
	if value := r.URL.Query().Get("pageSize"); value != "" {
		size, _ = strconv.Atoi(value)
	}
	result, err := s.groupChatQuery.SortedPage(r.PathValue("snapshotId"), page, size, r.URL.Query().Get("sortOrder"))
	s.writeHTTPErrorResult(w, 200, result, err)
}
