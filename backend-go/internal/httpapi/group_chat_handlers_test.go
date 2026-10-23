package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGroupChatCopyRequiresAdmin(t *testing.T) {
	s := &Server{}
	for _, target := range []struct{ method, path string }{
		{"GET", "/api/admin/group-chat/copy-schedule"}, {"PUT", "/api/admin/group-chat/copy-schedule"}, {"GET", "/api/admin/group-chat/local-files"}, {"POST", "/api/admin/group-chat/local-files/delete"}, {"GET", "/api/admin/group-chat/config"}, {"PUT", "/api/admin/group-chat/config"},
		{"POST", "/api/admin/group-chat/query"}, {"GET", "/api/admin/group-chat/query/snapshot"},
		{"GET", "/api/admin/group-chat/files"}, {"POST", "/api/admin/group-chat/copy-jobs"},
		{"GET", "/api/admin/group-chat/directory-files"},
		{"GET", "/api/admin/group-chat/copy-jobs/id"},
	} {
		called := false
		handler := s.authMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
		req := httptest.NewRequest(target.method, target.path, nil)
		req.Header.Set("X-Player-Stats-Key", "plugin-key")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if called || response.Code != 401 {
			t.Fatalf("%s %s bypassed admin authentication", target.method, target.path)
		}
	}
}
