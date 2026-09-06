package importer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// TodayPlayerPresence is a player occurrence found in a live snapshot of
// today's log. The snapshots are owned by Service.liveSnapshots, so this path
// shares the same cache and refresh coalescing used by log queries.
type TodayPlayerPresence struct {
	ServerID   string
	ServerName string
	PlayerName string
	FirstSeen  time.Time
}

// FindPlayerInTodayLogs looks for an exact player name in today's live logs.
// If no match is found, any snapshot/read error is returned so callers can
// distinguish "not found" from "could not verify".
func (s *Service) FindPlayerInTodayLogs(ctx context.Context, requestedServerID, requestedPlayerName string) ([]TodayPlayerPresence, error) {
	requestedPlayerName = strings.TrimSpace(requestedPlayerName)
	if requestedPlayerName == "" {
		return nil, nil
	}

	matches := make([]TodayPlayerPresence, 0)
	var scanErrors []error
	for _, source := range s.selectedSources(ctx, requestedServerID) {
		snapshot, err := s.acquireTodayLogSnapshot(ctx, source)
		if err != nil {
			scanErrors = append(scanErrors, fmt.Errorf("读取服务器 %s 的当天日志失败: %w", source.ID, err))
			continue
		}

		presence, found, scanErr := findPlayerInSnapshotFiles(ctx, snapshot.Files, requestedPlayerName, s.cfg.Location)
		snapshot.Release()
		if scanErr != nil {
			scanErrors = append(scanErrors, fmt.Errorf("扫描服务器 %s 的当天日志失败: %w", source.ID, scanErr))
			continue
		}
		if found {
			presence.ServerID = source.ID
			presence.ServerName = source.Name
			matches = append(matches, presence)
		}
	}

	if len(matches) > 0 {
		return matches, nil
	}
	if len(scanErrors) > 0 {
		return nil, errors.Join(scanErrors...)
	}
	return nil, nil
}

func findPlayerInSnapshotFiles(ctx context.Context, files []RemoteLogFile, requestedPlayerName string, location *time.Location) (TodayPlayerPresence, bool, error) {
	var earliest TodayPlayerPresence
	found := false
	for _, file := range files {
		match, fileFound, err := findPlayerInSnapshotFile(ctx, file, requestedPlayerName, location)
		if err != nil {
			return TodayPlayerPresence{}, false, err
		}
		if fileFound && (!found || (!match.FirstSeen.IsZero() && (earliest.FirstSeen.IsZero() || match.FirstSeen.Before(earliest.FirstSeen)))) {
			earliest = match
			found = true
		}
	}
	return earliest, found, nil
}

func findPlayerInSnapshotFile(ctx context.Context, file RemoteLogFile, requestedPlayerName string, location *time.Location) (TodayPlayerPresence, bool, error) {
	opened, err := os.Open(file.Path)
	if err != nil {
		return TodayPlayerPresence{}, false, err
	}
	defer opened.Close()

	var match TodayPlayerPresence
	scanner := bufio.NewScanner(opened)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return TodayPlayerPresence{}, false, err
		}
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		columns := splitPrefix(line)
		if isHeader(columns) {
			continue
		}
		name := playerName(columns)
		if !strings.EqualFold(name, requestedPlayerName) {
			continue
		}
		match.PlayerName = name
		if happenedAt, parsed := parseDateTime(columns, location); parsed {
			match.FirstSeen = happenedAt
		}
		return match, true, nil
	}
	if err := scanner.Err(); err != nil {
		return TodayPlayerPresence{}, false, err
	}
	return TodayPlayerPresence{}, false, nil
}
