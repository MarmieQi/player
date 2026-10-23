package groupchat

import (
	"path/filepath"
	"player-stats-backend-go/internal/config"
	"testing"
)

func TestNormalizeChatCopyConfig(t *testing.T) {
	c, err := NormalizeCopyConfig(CopyConfig{Enabled: true, SMBDirectory: ` chat\212 `})
	if err != nil || c.SMBDirectory != "chat/212" || c.FileGlob != "*.txt" {
		t.Fatalf("config = %#v, err = %v", c, err)
	}
	for _, c := range []CopyConfig{
		{Enabled: true}, {SMBDirectory: "../other"}, {SMBDirectory: "chat/../other"}, {SMBDirectory: "C:/chat"},
		{SMBDirectory: "chat", FileGlob: "*.csv"}, {SMBDirectory: "chat", FileGlob: "../*.txt"}, {SMBDirectory: "chat", FileGlob: "[.txt"},
	} {
		if _, err := NormalizeCopyConfig(c); err == nil {
			t.Errorf("accepted invalid config: %#v", c)
		}
	}
	if _, err := NormalizeCopyConfig(CopyConfig{}); err != nil {
		t.Fatalf("disabled default should be valid: %v", err)
	}
}

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
