package importer

import (
	"os"
	"strings"
	"testing"
)

func TestTextSnapshotPreservesRawCompleteLinesWithoutCSVHeader(t *testing.T) {
	raw := "\ufeff[2026-10-24 00:00:04] [主服][昵称]<Player With Spaces> 原样, 保留\r\n"
	for _, input := range []string{raw + "incomplete", raw, "", "only incomplete"} {
		root := t.TempDir()
		path, size, err := writeLiveSnapshot(strings.NewReader(input), int64(len(input)), root, "chat", false)
		if err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		expected := ""
		if strings.HasPrefix(input, raw) {
			expected = raw
		}
		if string(content) != expected || size != int64(len(expected)) {
			t.Fatalf("got %q size=%d", content, size)
		}
	}
}

func TestTextSnapshotRejectsShortRead(t *testing.T) {
	if _, _, err := writeLiveSnapshot(strings.NewReader("short\n"), 100, t.TempDir(), "chat", false); err == nil {
		t.Fatal("short read accepted")
	}
}
