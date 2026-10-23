package httpapi

import (
	"encoding/json"
	"net/http"

	"player-stats-backend-go/internal/settings"
)

func (s *Server) configSync(w http.ResponseWriter, r *http.Request) {
	response, err := s.settingsService.GetSyncConfig(r.Context())
	s.writeServiceResult(w, response, err)
}

func (s *Server) saveConfigSync(w http.ResponseWriter, r *http.Request) {
	var request settings.SyncConfigResponse
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "请求体不是有效 JSON")
		return
	}
	response, err := s.settingsService.SaveSyncConfig(r.Context(), request)
	s.writeServiceResult(w, response, err)
}

func (s *Server) astrBotKey(w http.ResponseWriter, r *http.Request) {
	response, err := s.settingsService.AstrBotKey(r.Context())
	s.writeServiceResult(w, response, err)
}

func (s *Server) resetAstrBotKey(w http.ResponseWriter, r *http.Request) {
	response, err := s.settingsService.ResetAstrBotKey(r.Context())
	s.writeServiceResult(w, response, err)
}

func (s *Server) smbTest(w http.ResponseWriter, r *http.Request) {
	message, err := s.importService.TestSMBConnection(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": message,
	})
}

func (s *Server) sourceFiles(w http.ResponseWriter, r *http.Request) {
	metadata, err := s.importService.ListSourceDirectoryFiles(r.Context(), r.URL.Query().Get("sourceId"))
	if err == nil {
		files := make([]settings.RemoteLogFile, 0, len(metadata))
		for _, status := range metadata {
			files = append(files, settings.RemoteLogFile{
				FileName:     status.FileName,
				Path:         status.Path,
				Size:         status.Size,
				LastModified: status.LastModified,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "files": files})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": false,
		"message": err.Error(),
		"files":   []any{},
	})
}
