package groupchat

import (
	"path/filepath"
	"player-stats-backend-go/internal/config"
	"testing"
)

func TestChatArchiveIsNotAStatisticsSource(t *testing.T) {
	cfg := config.Config{LogArchiveDirectory: t.TempDir(), MainDirectory: "main", SubDirectory: "sub", MainEnabled: true, SubEnabled: true}
	service := NewCopyService(nil, cfg, nil)
	if service.archiveRoot != filepath.Join(cfg.LogArchiveDirectory, "chat") {
		t.Fatal(service.archiveRoot)
	}
	for _, source := range cfg.Sources() {
		if source.ID == "group-chat" || source.Directory == service.archiveRoot {
			t.Fatal("chat archive leaked into import sources")
		}
	}
}
