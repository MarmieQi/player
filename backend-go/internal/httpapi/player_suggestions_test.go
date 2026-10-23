package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPlayerSuggestionsRequireAdminAuthentication(t *testing.T) {
	server := &Server{}
	called := false
	handler := server.authMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	request := httptest.NewRequest("GET", "/api/stats/player-suggestions?serverId=main&q=Alex", nil)
	request.Header.Set("X-Player-Stats-Key", "plugin-key")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || called {
		t.Fatalf("suggestions bypassed admin auth: %d", response.Code)
	}
}
