package httpapi

import (
	"encoding/json"
	"net/http"
	"player-stats-backend-go/internal/importer"
)

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
