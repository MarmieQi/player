package settings

import (
	"context"
	"path"
	"player-stats-backend-go/internal/auth"
	"player-stats-backend-go/internal/storage"
	"strings"
	"time"
)

// Each section is an independent save boundary. No write includes fields from
// another tab, and multi-table updates always share a transaction.
type SMBSettings struct {
	Host     string        `json:"host"`
	Port     int           `json:"port"`
	Domain   string        `json:"domain"`
	Username string        `json:"username"`
	Password string        `json:"password"`
	Share    string        `json:"share"`
	Sources  []SyncSource  `json:"sources"`
	Chat     ChatDirectory `json:"chat"`
}
type ChatSchedule struct {
	Enabled bool   `json:"enabled"`
	Time    string `json:"time"`
}
type ScheduleSettings struct {
	SkipToday bool              `json:"skipToday"`
	Tasks     []AutoTaskSetting `json:"tasks"`
	Chat      ChatSchedule      `json:"chat"`
}
type SharingSettings struct {
	ShareTTLMinutes         int `json:"shareTtlMinutes"`
	LogQueryShareTTLMinutes int `json:"logQueryShareTtlMinutes"`
}

func (s *Service) SMB(ctx context.Context) (SMBSettings, error) {
	v, e := s.SMBConnectionConfig(ctx)
	if e != nil {
		return SMBSettings{}, e
	}
	p := SMBSettings{Host: v.Host, Port: v.Port, Domain: v.Domain, Username: v.Username, Password: v.Password, Share: v.Share}
	if p.Password != "" {
		p.Password = "********"
	}
	p.Sources, e = s.Sources(ctx)
	if e != nil {
		return p, e
	}
	e = s.db.QueryRowContext(ctx, `select enabled,smb_directory,smb_file_glob from log_sources where source_id='group-chat'`).Scan(&p.Chat.Enabled, &p.Chat.SMBDirectory, &p.Chat.FileGlob)
	return p, e
}
func validateSMB(p *SMBSettings) error {
	if p.Port < 1 || p.Port > 65535 {
		return auth.NewHTTPError(400, "SMB 端口必须为 1～65535")
	}
	for _, v := range []string{p.Host, p.Domain, p.Username, p.Share} {
		if len(v) > 255 || strings.ContainsRune(v, 0) {
			return auth.NewHTTPError(400, "SMB 连接字段过长或含无效字符")
		}
	}
	if len(p.Password) > 255 {
		return auth.NewHTTPError(400, "SMB 密码过长")
	}
	chat, err := NormalizeChatDirectory(p.Chat)
	if err != nil {
		return err
	}
	p.Chat = chat
	if len(p.Sources) > 2 {
		return auth.NewHTTPError(400, "日志源配置数量无效")
	}
	seen := map[string]bool{}
	for i := range p.Sources {
		source := &p.Sources[i]
		if (source.SourceID != "main" && source.SourceID != "sub") || seen[source.SourceID] {
			return auth.NewHTTPError(400, "日志源必须是主服和 2服，且不能重复")
		}
		seen[source.SourceID] = true
		source.SMBDirectory, err = normalizeRelativeDirectory(source.SMBDirectory)
		if err != nil {
			return err
		}
		source.SMBFileGlob = strings.TrimSpace(source.SMBFileGlob)
		if source.SMBFileGlob == "" {
			source.SMBFileGlob = "player_actions_*.csv"
		}
		if len(source.SMBFileGlob) > 128 || strings.ContainsAny(source.SMBFileGlob, "/\\\x00") || !strings.HasSuffix(source.SMBFileGlob, ".csv") {
			return auth.NewHTTPError(400, "CSV 文件匹配必须为 CSV 文件名规则")
		}
		if _, err = path.Match(source.SMBFileGlob, "test.csv"); err != nil {
			return auth.NewHTTPError(400, "CSV 文件匹配规则无效")
		}
		if len(source.SourceName) > 64 {
			return auth.NewHTTPError(400, "服务器名称过长")
		}
	}
	return nil
}
func (s *Service) SaveSMB(ctx context.Context, p SMBSettings) (SMBSettings, error) {
	if e := validateSMB(&p); e != nil {
		return p, e
	}
	if e := s.db.Write(ctx, func(tx *storage.Tx) error { return saveSMBTx(ctx, tx, p) }); e != nil {
		return p, e
	}
	return s.SMB(ctx)
}
func saveSMBTx(ctx context.Context, tx *storage.Tx, p SMBSettings) error {
	var password string
	if e := tx.QueryRowContext(ctx, `select password from smb_connections where id=1`).Scan(&password); e != nil {
		return e
	}
	if strings.TrimSpace(p.Password) != "" && p.Password != "********" {
		password = p.Password
	}
	rows, e := tx.QueryContext(ctx, `select source_id from log_sources where kind='csv'`)
	if e != nil {
		return e
	}
	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if e := rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids[id] = true
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if len(ids) != len(p.Sources) {
		return auth.NewHTTPError(400, "日志源配置已变化，请重新加载")
	}
	for _, v := range p.Sources {
		if !ids[v.SourceID] {
			return auth.NewHTTPError(400, "未知日志源")
		}
	}
	if _, e := tx.ExecContext(ctx, `update smb_connections set host=?,port=?,domain=?,username=?,password=?,share=? where id=1`, strings.TrimSpace(p.Host), p.Port, p.Domain, p.Username, password, strings.TrimSpace(p.Share)); e != nil {
		return e
	}
	for _, v := range p.Sources {
		if _, e := tx.ExecContext(ctx, `update log_sources set source_name=?,smb_directory=?,smb_file_glob=?,smb_recursive=?,enabled=? where source_id=? and kind='csv'`, v.SourceName, v.SMBDirectory, v.SMBFileGlob, v.SMBRecursive, v.Enabled, v.SourceID); e != nil {
			return e
		}
	}
	_, e = tx.ExecContext(ctx, `update log_sources set enabled=?,smb_directory=?,smb_file_glob=? where source_id='group-chat'`, p.Chat.Enabled, p.Chat.SMBDirectory, p.Chat.FileGlob)
	return e
}
func normalizeRelativeDirectory(value string) (string, error) {
	v := strings.Trim(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "/")
	if len(v) > 512 || strings.ContainsAny(v, ":\x00") {
		return "", auth.NewHTTPError(400, "请填写 SMB 共享内的相对目录")
	}
	for _, part := range strings.Split(v, "/") {
		if part == ".." {
			return "", auth.NewHTTPError(400, "目录不能包含上级路径")
		}
	}
	v = path.Clean(v)
	if v == "." {
		v = ""
	}
	return v, nil
}

func (s *Service) Schedules(ctx context.Context) (ScheduleSettings, error) {
	p := ScheduleSettings{Tasks: []AutoTaskSetting{}}
	var e error
	p.SkipToday, e = s.SkipToday(ctx)
	if e != nil {
		return p, e
	}
	rows, e := s.db.QueryContext(ctx, `select s.source_id,s.source_name,t.kind,t.enabled,t.run_time from task_schedules t join log_sources s on s.source_id=t.source_id order by s.source_id,t.kind`)
	if e != nil {
		return p, e
	}
	defer rows.Close()
	tasks := map[string]*AutoTaskSetting{}
	for rows.Next() {
		var id, name, kind, t string
		var enabled bool
		if e := rows.Scan(&id, &name, &kind, &enabled, &t); e != nil {
			return p, e
		}
		if id == "group-chat" {
			p.Chat = ChatSchedule{enabled, t}
			continue
		}
		v := tasks[id]
		if v == nil {
			v = &AutoTaskSetting{ServerID: id, ServerName: name}
			tasks[id] = v
		}
		if kind == "SYNC" {
			v.SyncEnabled = enabled
			v.SyncTime = t
		} else {
			v.ImportEnabled = enabled
			v.ImportTime = t
		}
	}
	for _, id := range []string{"main", "sub"} {
		if v := tasks[id]; v != nil {
			p.Tasks = append(p.Tasks, *v)
		}
	}
	return p, rows.Err()
}
func validScheduleTime(v string) bool {
	if len(v) != 5 {
		return false
	}
	_, e := time.Parse("15:04", v)
	return e == nil
}
func validatePlans(p ScheduleSettings) error {
	if !validScheduleTime(p.Chat.Time) {
		return auth.NewHTTPError(400, "同步时间无效")
	}
	seen := map[string]bool{}
	for _, v := range p.Tasks {
		if (v.ServerID != "main" && v.ServerID != "sub") || seen[v.ServerID] || !validScheduleTime(v.SyncTime) || !validScheduleTime(v.ImportTime) {
			return auth.NewHTTPError(400, "任务配置无效")
		}
		seen[v.ServerID] = true
	}
	return nil
}
func (s *Service) SaveSchedules(ctx context.Context, p ScheduleSettings) (ScheduleSettings, error) {
	if e := validatePlans(p); e != nil {
		return p, e
	}
	if e := s.db.Write(ctx, func(tx *storage.Tx) error { return savePlansTx(ctx, tx, p) }); e != nil {
		return p, e
	}
	return s.Schedules(ctx)
}
func savePlansTx(ctx context.Context, tx *storage.Tx, p ScheduleSettings) error {
	if p.Chat.Enabled {
		var enabled bool
		var dir string
		if e := tx.QueryRowContext(ctx, `select enabled,smb_directory from log_sources where source_id='group-chat'`).Scan(&enabled, &dir); e != nil {
			return e
		}
		if !enabled || strings.TrimSpace(dir) == "" {
			return auth.NewHTTPError(400, "请先启用并保存群服聊天的 SMB 目录")
		}
	}
	if _, e := tx.ExecContext(ctx, `update ingest_policy set skip_today=? where id=1`, p.SkipToday); e != nil {
		return e
	}
	for _, v := range p.Tasks {
		for _, t := range []struct {
			k string
			e bool
			t string
		}{{"SYNC", v.SyncEnabled, v.SyncTime}, {"IMPORT", v.ImportEnabled, v.ImportTime}} {
			if _, e := tx.ExecContext(ctx, `update task_schedules set enabled=?,run_time=? where source_id=? and kind=?`, t.e, t.t, v.ServerID, t.k); e != nil {
				return e
			}
		}
	}
	_, e := tx.ExecContext(ctx, `update task_schedules set enabled=?,run_time=? where source_id='group-chat' and kind='SYNC'`, p.Chat.Enabled, p.Chat.Time)
	return e
}
func (s *Service) Sharing(ctx context.Context) (SharingSettings, error) {
	var p SharingSettings
	e := s.db.QueryRowContext(ctx, `select share_ttl_minutes,log_query_share_ttl_minutes from sharing_settings where id=1`).Scan(&p.ShareTTLMinutes, &p.LogQueryShareTTLMinutes)
	return p, e
}
func validateSharing(p SharingSettings) error {
	if p.ShareTTLMinutes < 5 || p.ShareTTLMinutes > 10080 || p.LogQueryShareTTLMinutes < 5 || p.LogQueryShareTTLMinutes > 10080 {
		return auth.NewHTTPError(400, "分享有效期必须为 5～10080 分钟")
	}
	return nil
}
func saveSharingTx(ctx context.Context, tx *storage.Tx, p SharingSettings) error {
	_, e := tx.ExecContext(ctx, `update sharing_settings set share_ttl_minutes=?,log_query_share_ttl_minutes=? where id=1`, p.ShareTTLMinutes, p.LogQueryShareTTLMinutes)
	return e
}
func (s *Service) SaveSharing(ctx context.Context, p SharingSettings) (SharingSettings, error) {
	if e := validateSharing(p); e != nil {
		return p, e
	}
	e := s.db.Write(ctx, func(tx *storage.Tx) error { return saveSharingTx(ctx, tx, p) })
	return p, e
}
