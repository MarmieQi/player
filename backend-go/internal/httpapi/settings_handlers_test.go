package httpapi

import (
	"net/http"
	"net/http/httptest"
	"player-stats-backend-go/internal/config"
	"strings"
	"testing"
)

func TestSettingsRequireAdmin(t *testing.T) {
	s := &Server{}
	for _, section := range []string{"smb", "schedules", "sharing", "runtime"} {
		for _, method := range []string{"GET", "PUT"} {
			called := false
			handler := s.authMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
			req := httptest.NewRequest(method, "/api/admin/settings/"+section, nil)
			req.Header.Set("X-Player-Stats-Key", "plugin-key")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if called || w.Code != 401 {
				t.Fatalf("settings access bypassed: %s %s", method, section)
			}
		}
	}
}
func TestDecodeSettingsRejectsUnknownAndTrailingData(t *testing.T) {
	for _, body := range []string{`{"liveCacheSeconds":60,"secret":"ignored"}`, `{} {}`, `{"liveCacheSeconds":60.5}`, strings.Repeat("x", 32769)} {
		r := httptest.NewRequest("PUT", "/api/admin/settings/runtime", strings.NewReader(body))
		w := httptest.NewRecorder()
		var p config.RuntimeParameters
		if decodeSettings(w, r, &p) || w.Code != 400 {
			t.Fatal("invalid configuration accepted")
		}
	}
}
