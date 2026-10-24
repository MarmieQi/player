package httpapi

import (
	"net/http"

	"player-stats-backend-go/internal/importer"
)

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
		files := metadata
		if files == nil {
			files = []importer.RemoteLogFile{}
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
