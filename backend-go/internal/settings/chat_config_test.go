package settings

import "testing"

func TestNormalizeChatDirectory(t *testing.T) {
	c, err := NormalizeChatDirectory(ChatDirectory{Enabled: true, SMBDirectory: ` chat\212 `})
	if err != nil || c.SMBDirectory != "chat/212" || c.FileGlob != "*.txt" {
		t.Fatalf("config = %#v, err = %v", c, err)
	}
	for _, c := range []ChatDirectory{
		{Enabled: true}, {SMBDirectory: "../other"}, {SMBDirectory: "chat/../other"}, {SMBDirectory: "C:/chat"},
		{SMBDirectory: "chat", FileGlob: "*.csv"}, {SMBDirectory: "chat", FileGlob: "../*.txt"}, {SMBDirectory: "chat", FileGlob: "[.txt"},
	} {
		if _, err := NormalizeChatDirectory(c); err == nil {
			t.Errorf("accepted invalid config: %#v", c)
		}
	}
	if _, err := NormalizeChatDirectory(ChatDirectory{}); err != nil {
		t.Fatalf("disabled default should be valid: %v", err)
	}
}
