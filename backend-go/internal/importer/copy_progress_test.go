package importer

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"player-stats-backend-go/internal/config"
)

func TestCopyUnchangedAndLimitedFilesNeverLeaveRunningRows(t *testing.T) {
	for _, extension := range []string{"csv", "txt"} {
		t.Run(extension, func(t *testing.T) {
			service := &Service{cfg: config.Config{Location: time.UTC, MaxFilesPerRun: 1}}
			source := config.Source{ID: "main", Name: "主服", Directory: t.TempDir()}
			modified := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			payload := []byte("raw file")
			remote := []RemoteLogFile{}
			for _, day := range []string{"01", "02", "03"} {
				remote = append(remote, RemoteLogFile{FileName: "2020-01-" + day + "." + extension, Size: int64(len(payload)), LastModified: modified})
			}
			first := filepath.Join(source.Directory, remote[0].FileName)
			if err := writeArchiveFile(bytes.NewReader(payload), first, modified, int64(len(payload))); err != nil {
				t.Fatal(err)
			}
			for pass := 0; pass < 2; pass++ {
				state := newJobState(nil)
				state.markRunning()
				copies := 0
				results, err := service.copyArchiveFiles(context.Background(), remote, source, false, state, func(file RemoteLogFile) (bool, error) {
					copies++
					running := 0
					for _, row := range state.view().Files {
						if row.Status == "RUNNING" {
							running++
						}
					}
					if running != 1 {
						t.Fatalf("copying with %d running rows", running)
					}
					return true, writeArchiveFile(bytes.NewReader(payload), file.Path, file.LastModified, file.Size)
				})
				if err != nil || copies != 1 {
					t.Fatalf("pass %d copies %d: %v", pass, copies, err)
				}
				state.markFinished(summarizeRun(time.Now(), results))
				view := state.view()
				if view.ImportedFiles != 1 || view.SkippedFiles != pass+1 {
					t.Fatalf("bad counters: %+v", view)
				}
				for _, row := range view.Files {
					if row.Status == "RUNNING" {
						t.Fatalf("terminal task contains running file: %+v", row)
					}
				}
				if pass == 0 && len(view.Files) != 2 {
					t.Fatal("unprocessed third file was reported as started")
				}
			}
		})
	}
}

func TestFailedCopyFinalizesActiveFileAndRetainsCompletedResults(t *testing.T) {
	state := newJobState(nil)
	state.markRunning()
	one := RemoteLogFile{Path: "/data/main/one.csv", FileName: "one.csv"}
	two := RemoteLogFile{Path: "/data/main/two.csv", FileName: "two.csv"}
	state.FileStarted("main", "主服", one)
	state.FileFinished(copiedResult("main", "主服", one.Path))
	state.FileStarted("main", "主服", two)
	state.markFailed("connection interrupted")
	view := state.view()
	if view.Status != "FAILED" || view.ImportedFiles != 1 || view.FailedFiles != 1 || view.ScannedFiles != 2 {
		t.Fatal(view)
	}
	if view.Files[0].Status != "COPIED" || view.Files[1].Status != "FAILED" || view.Files[1].FinishedAt == nil {
		t.Fatal(view.Files)
	}
}
