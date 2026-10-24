package settings

import "testing"

func TestSettingsDirectoryAndScheduleValidation(t *testing.T) {
	for _, value := range []string{"../outside", `logs\..\outside`, "C:/outside", "logs\x00bad"} {
		if _, err := normalizeRelativeDirectory(value); err == nil {
			t.Fatalf("unsafe directory accepted: %q", value)
		}
	}
	if value, err := normalizeRelativeDirectory(` logs\chat `); err != nil || value != "logs/chat" {
		t.Fatalf("relative directory normalization: %q %v", value, err)
	}
	for _, value := range []string{"00:00", "23:59", "01:05"} {
		if !validScheduleTime(value) {
			t.Fatalf("valid time rejected: %q", value)
		}
	}
	for _, value := range []string{"24:00", "00:60", "1:05", "12:3", "12:30:00"} {
		if validScheduleTime(value) {
			t.Fatalf("invalid time accepted: %q", value)
		}
	}
}
