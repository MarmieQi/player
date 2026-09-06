package importer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindPlayerInSnapshotFilesMatchesExactNameIgnoringCase(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	path := writePlayerPresenceTestLog(t,
		testLogHeader+
			"2026-09-06,16:00:00,Steve2,登录,0,0,0,overworld,-,-,-,-,-,-\r\n"+
			"2026-09-06,16:01:00,STEVE,登录,0,0,0,overworld,-,-,-,-,-,-\r\n")

	match, found, err := findPlayerInSnapshotFiles(context.Background(), []RemoteLogFile{{Path: path}}, "steve", location)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected exact case-insensitive player match")
	}
	if match.PlayerName != "STEVE" {
		t.Fatalf("player name = %q, want %q", match.PlayerName, "STEVE")
	}
	if got := match.FirstSeen.Format("2006-01-02 15:04:05"); got != "2026-09-06 16:01:00" {
		t.Fatalf("first seen = %q", got)
	}
}

func TestFindPlayerInSnapshotFilesReturnsEarliestOccurrence(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	late := writePlayerPresenceTestLog(t, testLogHeader+"2026-09-06,18:00:00,Alex,登录,0,0,0,overworld,-,-,-,-,-,-\r\n")
	early := writePlayerPresenceTestLog(t, testLogHeader+"2026-09-06,09:00:00,Alex,登录,0,0,0,overworld,-,-,-,-,-,-\r\n")

	match, found, err := findPlayerInSnapshotFiles(context.Background(), []RemoteLogFile{{Path: late}, {Path: early}}, "Alex", location)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected player match")
	}
	if got := match.FirstSeen.Format("15:04:05"); got != "09:00:00" {
		t.Fatalf("first seen = %q, want 09:00:00", got)
	}
}

func TestFindPlayerInSnapshotFilesDoesNotUseSubstringMatch(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	path := writePlayerPresenceTestLog(t, testLogHeader+"2026-09-06,16:00:00,Steve2,登录,0,0,0,overworld,-,-,-,-,-,-\r\n")

	_, found, err := findPlayerInSnapshotFiles(context.Background(), []RemoteLogFile{{Path: path}}, "Steve", location)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("substring player name unexpectedly matched")
	}
}

func TestFindPlayerInSnapshotFilesStopsWhenContextIsCancelled(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	path := writePlayerPresenceTestLog(t, testLogHeader+"2026-09-06,16:00:00,Steve,登录,0,0,0,overworld,-,-,-,-,-,-\r\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, found, err := findPlayerInSnapshotFiles(ctx, []RemoteLogFile{{Path: path}}, "Steve", location)
	if err == nil {
		t.Fatal("expected cancelled context error")
	}
	if found {
		t.Fatal("cancelled scan unexpectedly returned a match")
	}
}

func writePlayerPresenceTestLog(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "player_actions_2026-09-06.csv")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
