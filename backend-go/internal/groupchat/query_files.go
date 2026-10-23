package groupchat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"player-stats-backend-go/internal/auth"
)

type queryFile struct {
	path, name string
	day        time.Time
}
type queryFiles struct {
	files   []queryFile
	missing []string
	release func()
}

func fileDay(name string, zone *time.Location) (time.Time, bool) {
	// Chat archives use daily YYYY-MM-DD.txt names; no directory comes from input.
	day, err := time.ParseInLocation("2006-01-02", strings.TrimSuffix(name, ".txt"), zone)
	return day, err == nil
}

func (s *CopyService) queryFiles(ctx context.Context, from, to time.Time) (queryFiles, error) {
	if err := ctx.Err(); err != nil {
		return queryFiles{}, err
	}
	source, err := s.archiveSource(ctx, false)
	if err != nil {
		return queryFiles{}, err
	}
	result := queryFiles{release: func() {}, missing: []string{}}
	today, _ := time.ParseInLocation("2006-01-02", time.Now().In(from.Location()).Format("2006-01-02"), from.Location())
	present := map[string]bool{}
	entries, err := os.ReadDir(source.Local.Directory)
	if err != nil && !os.IsNotExist(err) {
		return result, err
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			continue
		}
		match, _ := filepath.Match(source.FileGlob, entry.Name())
		day, ok := fileDay(entry.Name(), from.Location())
		if !match || !ok || day.Before(from) || day.After(to) || day.Equal(today) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return result, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		result.files = append(result.files, queryFile{filepath.Join(source.Local.Directory, entry.Name()), entry.Name(), day})
		present[day.Format("2006-01-02")] = true
	}
	if !today.Before(from) && !today.After(to) {
		enabled, err := s.source(ctx)
		if err != nil {
			return result, err
		}
		live, err := s.copier.AcquireTodayTextSnapshot(ctx, enabled)
		if err != nil {
			return result, auth.NewHTTPError(502, fmt.Sprintf("刷新当天聊天失败，请重试：%v", err))
		}
		if time.Now().In(from.Location()).Format("2006-01-02") != today.Format("2006-01-02") {
			live.Release()
			return s.queryFiles(ctx, from, to)
		}
		result.release = live.Release
		for _, file := range live.Files {
			day, ok := fileDay(file.FileName, from.Location())
			if !ok || !day.Equal(today) {
				continue
			}
			result.files = append(result.files, queryFile{file.Path, file.FileName, day})
			present[day.Format("2006-01-02")] = true
		}
	}
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		value := day.Format("2006-01-02")
		if !present[value] {
			result.missing = append(result.missing, value)
		}
	}
	return result, nil
}
