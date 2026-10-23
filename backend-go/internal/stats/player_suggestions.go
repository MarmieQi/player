package stats

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"player-stats-backend-go/internal/auth"
)

// PlayerSuggestions reads the existing per-server directory, never the CSV files.
func (s *Service) PlayerSuggestions(ctx context.Context, serverID, query string) ([]string, error) {
	serverID, query = strings.TrimSpace(serverID), strings.TrimSpace(query)
	if serverID == "" || normalizeServerID(serverID) == "" || utf8.RuneCountInString(serverID) > 64 {
		return nil, auth.NewHTTPError(400, "请选择服务器")
	}
	if utf8.RuneCountInString(query) > 64 {
		return nil, auth.NewHTTPError(400, "玩家名搜索内容不能超过 64 个字符")
	}
	result := make([]string, 0, 10)
	if query == "" {
		return result, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	prefix := escapePlayerSuggestionLike(query) + "%"
	// Fill from an index-friendly prefix lookup before considering substring matches.
	for pass := 0; pass < 2 && len(result) < 10; pass++ {
		sql := `select player_name from player_server_profiles where server_id = ? and player_name like ? escape '!'`
		args := []any{serverID, prefix}
		if pass == 1 {
			sql += ` and player_name not like ? escape '!'`
			args = []any{serverID, "%" + escapePlayerSuggestionLike(query) + "%", prefix}
		}
		sql += ` order by player_name limit ?`
		args = append(args, 10-len(result))
		rows, err := s.db.QueryContext(ctx, sql, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return nil, err
			}
			result = append(result, name)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func escapePlayerSuggestionLike(value string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(value)
}
