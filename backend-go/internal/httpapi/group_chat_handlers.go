package httpapi

import (
	"encoding/json"
	"net/http"
	"player-stats-backend-go/internal/groupchat"
	"player-stats-backend-go/internal/importer"
)

func (s *Server) groupChatConfig(w http.ResponseWriter, r *http.Request) {
	result, err := s.groupChatCopy.Config(r.Context())
	s.writeServiceResult(w, result, err)
}
func (s *Server) saveGroupChatConfig(w http.ResponseWriter, r *http.Request) {
	var request groupchat.CopyConfig
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&request); err != nil {
		writeError(w, 400, "请求体不是有效配置")
		return
	}
	result, err := s.groupChatCopy.SaveConfig(r.Context(), request)
	s.writeServiceResult(w, result, err)
}
func (s *Server) groupChatFiles(w http.ResponseWriter, r *http.Request) {
	result, err := s.groupChatCopy.Files(r.Context(), queryBool(r, "skipToday", true))
	s.writeHTTPErrorResult(w, 200, result, err)
}
func (s *Server) startGroupChatCopy(w http.ResponseWriter, r *http.Request) {
	result, err := s.groupChatCopy.Start(r.Context(), queryBool(r, "skipToday", true))
	s.writeHTTPErrorResult(w, 202, result, err)
}
func (s *Server) groupChatDirectoryFiles(w http.ResponseWriter, r *http.Request) {
	result, err := s.groupChatCopy.DirectoryFiles(r.Context())
	s.writeHTTPErrorResult(w, 200, result, err)
}
func (s *Server) groupChatCopyJob(w http.ResponseWriter, r *http.Request) {
	result, err := s.groupChatCopy.Job(r.PathValue("jobId"))
	s.writeHTTPErrorResult(w, 200, result, err)
}

func (s *Server) groupChatSchedule(w http.ResponseWriter, r *http.Request) {
	result, err := s.groupChatCopy.Schedule(r.Context())
	s.writeServiceResult(w, result, err)
}
func (s *Server) saveGroupChatSchedule(w http.ResponseWriter, r *http.Request) {
	var request groupchat.CopySchedule
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&request); err != nil {
		writeError(w, 400, "请求体不是有效配置")
		return
	}
	result, err := s.groupChatCopy.SaveSchedule(r.Context(), request)
	s.writeServiceResult(w, result, err)
}

func (s *Server) groupChatLocalFiles(w http.ResponseWriter, r *http.Request) {
	result, err := s.groupChatCopy.LocalFiles(r.Context())
	s.writeHTTPErrorResult(w, 200, result, err)
}
func (s *Server) deleteGroupChatLocalFiles(w http.ResponseWriter, r *http.Request) {
	var request importer.DeleteImportRecordsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1048576)).Decode(&request); err != nil {
		writeError(w, 400, "请求体不是有效 JSON")
		return
	}
	result, err := s.groupChatCopy.DeleteLocalFiles(r.Context(), request)
	s.writeHTTPErrorResult(w, 200, result, err)
}
