package importer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"player-stats-backend-go/internal/config"
)

func TestRemoteFileStatusComparesLocalArchive(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	dir := t.TempDir()
	service := NewService(nil, config.Config{Location: location}, nil)
	source := config.Source{ID: "sub", Name: "2服", Directory: dir, FileGlob: "player_actions_*.csv", Enabled: true}
	modified := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	remote := RemoteLogFile{
		Path:         "logs/sub/player_actions_2026-04-24.csv",
		FileName:     "player_actions_2026-04-24.csv",
		Size:         6,
		LastModified: modified,
		SourcePath:   "logs/sub/player_actions_2026-04-24.csv",
	}
	archived := toArchivedSMBFile(source, remote)

	pending := service.remoteFileStatus(source, remote, archived, true)
	if pending.Status != "REMOTE" {
		t.Fatalf("empty archive status = %s, want REMOTE", pending.Status)
	}
	if pending.RemotePath != remote.Path {
		t.Fatalf("remote path = %q, want %q", pending.RemotePath, remote.Path)
	}
	if pending.LocalPath != archived.Path {
		t.Fatalf("local path = %q, want %q", pending.LocalPath, archived.Path)
	}

	if err := os.WriteFile(archived.Path, []byte("abc"), 0644); err != nil {
		t.Fatal(err)
	}
	changed := service.remoteFileStatus(source, remote, archived, true)
	if changed.Status != "CHANGED" {
		t.Fatalf("size mismatch status = %s, want CHANGED", changed.Status)
	}

	if err := os.WriteFile(archived.Path, []byte("abcdef"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(archived.Path, modified, modified); err != nil {
		t.Fatal(err)
	}
	copied := service.remoteFileStatus(source, remote, archived, true)
	if copied.Status != "COPIED" {
		t.Fatalf("matching archive status = %s, want COPIED", copied.Status)
	}
}

func TestWriteArchiveFileIgnoresStaleFixedTmpFile(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "player_actions_2026-04-24.csv")
	staleTmp := localPath + ".tmp"
	if err := os.WriteFile(staleTmp, []byte("stale"), 0400); err != nil {
		t.Fatal(err)
	}
	modified := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	if err := writeArchiveFile(strings.NewReader("fresh"), localPath, modified); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(staleTmp); !os.IsNotExist(err) {
		t.Fatalf("stale fixed tmp still exists or stat failed: %v", err)
	}
	content, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "fresh" {
		t.Fatalf("content = %q, want fresh", content)
	}
	info, err := os.Stat(localPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.ModTime().UTC().UnixMilli() != modified.UnixMilli() {
		t.Fatalf("mtime = %s, want %s", info.ModTime().UTC(), modified)
	}
}

func TestRemoteFileStatusUsesExplicitSkipTodaySetting(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	service := NewService(nil, config.Config{Location: location}, nil)
	source := config.Source{ID: "main", Directory: t.TempDir()}
	future := time.Now().In(location).AddDate(0, 0, 7)
	remote := RemoteLogFile{FileName: "player_actions_" + future.Format("2006-01-02") + ".csv", LastModified: future}
	archived := toArchivedSMBFile(source, remote)
	if got := service.remoteFileStatus(source, remote, archived, true).Status; got != "SKIPPED_TODAY" {
		t.Fatalf("skip enabled status = %s", got)
	}
	if got := service.remoteFileStatus(source, remote, archived, false).Status; got != "REMOTE" {
		t.Fatalf("skip disabled status = %s", got)
	}
}

func TestChatCopyPreservesRawBytesAndDateSkipping(t *testing.T) {
	now := time.Now().UTC()
	oldDay := now.AddDate(0, 0, -1).Format("2006-01-02")
	today := now.Format("2006-01-02")
	payload := []byte("\xef\xbb\xbf[2026-10-22 00:35:41] [主服][昵称]<Player With Spaces> hello, world\r\n")
	service := &Service{cfg: config.Config{Location: time.UTC}}
	source := config.Source{ID: "group-chat", Name: "群服聊天", Directory: filepath.Join(t.TempDir(), "chat"), FileGlob: "*.txt"}
	remote := []RemoteLogFile{
		{FileName: oldDay + ".txt", Size: int64(len(payload)), LastModified: now, SourcePath: "chat/212/" + oldDay + ".txt"},
		{FileName: today + ".txt", Size: int64(len(payload)), LastModified: now, SourcePath: "chat/212/" + today + ".txt"},
	}
	copies := 0
	results, err := service.copyArchiveFiles(context.Background(), remote, source, true, nil, func(file RemoteLogFile) (bool, error) {
		copies++
		if currentArchiveFile(file.Path, file) {
			return false, nil
		}
		return true, writeArchiveFile(bytes.NewReader(payload), file.Path, file.LastModified, file.Size)
	})
	if err != nil || copies != 1 || len(results) != 2 || results[0].Status != "COPIED" || results[1].Status != "SKIPPED" {
		t.Fatalf("results=%#v copies=%d err=%v", results, copies, err)
	}
	content, err := os.ReadFile(filepath.Join(source.Directory, oldDay+".txt"))
	if err != nil || !bytes.Equal(content, payload) {
		t.Fatalf("raw TXT changed: %q %v", content, err)
	}
	status := service.remoteFileStatus(source, remote[0], toArchivedSMBFile(source, remote[0]), true)
	if status.Status != "COPIED" {
		t.Fatal(status.Status)
	}
	remote[0].Size++
	if got := service.remoteFileStatus(source, remote[0], toArchivedSMBFile(source, remote[0]), true); got.Status != "CHANGED" {
		t.Fatal(got.Status)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.copyArchiveFiles(ctx, remote, source, false, nil, func(RemoteLogFile) (bool, error) { t.Fatal("cancelled copy continued"); return false, nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestIncompleteCopyDoesNotReplacePreviousArchive(t *testing.T) {
	target := filepath.Join(t.TempDir(), "2026-10-22.txt")
	if err := os.WriteFile(target, []byte("previous"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeArchiveFile(strings.NewReader("short"), target, time.Now(), 100); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "previous" {
		t.Fatalf("previous file overwritten: %q %v", data, err)
	}
	leftovers, _ := filepath.Glob(target + ".*.tmp")
	if len(leftovers) != 0 {
		t.Fatal(leftovers)
	}
}

func TestRawArchiveLocalMetadataAndConfinedDelete(t *testing.T) {
	root := t.TempDir()
	chat := filepath.Join(root, "chat")
	if err := os.MkdirAll(chat, 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(chat, "2026-10-23.txt")
	other := filepath.Join(root, "outside.txt")
	csv := filepath.Join(chat, "actions.csv")
	for _, name := range []string{target, other, csv} {
		if err := os.WriteFile(name, []byte("raw data"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	source := ArchiveCopySource{Local: config.Source{ID: "group-chat", Name: "群服聊天", Directory: chat, FileGlob: "*.txt"}}
	service := &Service{}
	files, err := service.ListLocalSMBArchive(source)
	if err != nil || len(files) != 1 || files[0].FileSize != 8 || files[0].Imported {
		t.Fatalf("local metadata = %#v err=%v", files, err)
	}
	for _, name := range []string{other, csv} {
		if _, err := service.DeleteSMBArchiveFile(source, name); err == nil {
			t.Fatalf("deleted disallowed file %s", name)
		}
	}
	for _, name := range []string{other, csv} {
		if _, err := os.Stat(name); err != nil {
			t.Fatal("disallowed file changed", err)
		}
	}
	result, err := service.DeleteSMBArchiveFile(source, target)
	if err != nil || !result.Deleted {
		t.Fatalf("delete=%#v err=%v", result, err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("local TXT still exists", err)
	}
	files, err = service.ListLocalSMBArchive(source)
	if err != nil || len(files) != 0 {
		t.Fatal(files, err)
	}
}
