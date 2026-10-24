package settings

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/storage"
	"strings"
)

const AstrBotAPIKeyHeader = "X-Player-Stats-Key"

type Service struct {
	db  *storage.DB
	cfg config.Config
}

func NewService(db *storage.DB, cfg config.Config) *Service {
	return &Service{db: db, cfg: cfg}
}

type SyncConfigResponse struct {
	SMBHost                 string            `json:"smbHost"`
	SMBPort                 int               `json:"smbPort"`
	SMBDomain               string            `json:"smbDomain"`
	SMBUsername             string            `json:"smbUsername"`
	SMBPassword             string            `json:"smbPassword"`
	SMBShare                string            `json:"smbShare"`
	AutoRun                 bool              `json:"autoRun"`
	SyncCron                string            `json:"syncCron"`
	SkipToday               bool              `json:"skipToday"`
	ShareTTLMinutes         int               `json:"shareTtlMinutes"`
	LogQueryShareTTLMinutes int               `json:"logQueryShareTtlMinutes"`
	Sources                 []SyncSource      `json:"sources"`
	AutoTasks               []AutoTaskSetting `json:"autoTasks"`
}

type SyncSource struct {
	ID           *int64 `json:"id"`
	SourceID     string `json:"sourceId"`
	SourceName   string `json:"sourceName"`
	SMBDirectory string `json:"smbDirectory"`
	SMBFileGlob  string `json:"smbFileGlob"`
	SMBRecursive bool   `json:"smbRecursive"`
	Enabled      bool   `json:"enabled"`
}

type SMBConnectionConfig struct {
	Host     string
	Port     int
	Domain   string
	Username string
	Password string
	Share    string
}

type AutoTaskSetting struct {
	ServerID      string `json:"serverId"`
	ServerName    string `json:"serverName"`
	SyncEnabled   bool   `json:"syncEnabled"`
	SyncTime      string `json:"syncTime"`
	ImportEnabled bool   `json:"importEnabled"`
	ImportTime    string `json:"importTime"`
}

type AstrBotKeyResponse struct {
	APIKey     string `json:"apiKey"`
	HeaderName string `json:"headerName"`
}

type RemoteLogFile struct {
	FileName     string `json:"fileName"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	LastModified any    `json:"lastModified"`
}

func (s *Service) Sources(ctx context.Context) ([]SyncSource, error) {
	rows, err := s.db.QueryContext(ctx, `select case source_id when 'main' then 1 else 2 end,source_id,source_name,smb_directory,smb_file_glob,smb_recursive,enabled from log_sources where kind='csv' order by source_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SyncSource{}
	for rows.Next() {
		var v SyncSource
		var id int64
		if err := rows.Scan(&id, &v.SourceID, &v.SourceName, &v.SMBDirectory, &v.SMBFileGlob, &v.SMBRecursive, &v.Enabled); err != nil {
			return nil, err
		}
		v.ID = &id
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) SourceByID(ctx context.Context, id string) (SyncSource, bool, error) {
	all, err := s.Sources(ctx)
	if err != nil {
		return SyncSource{}, false, err
	}
	for _, v := range all {
		if v.SourceID == id {
			return v, true, nil
		}
	}
	return SyncSource{}, false, nil
}
func (s *Service) SMBConnectionConfig(ctx context.Context) (SMBConnectionConfig, error) {
	var v SMBConnectionConfig
	e := s.db.QueryRowContext(ctx, `select host,port,domain,username,password,share from smb_connections where id=1`).Scan(&v.Host, &v.Port, &v.Domain, &v.Username, &v.Password, &v.Share)
	return v, e
}
func (s *Service) SkipToday(ctx context.Context) (bool, error) {
	var v bool
	e := s.db.QueryRowContext(ctx, `select skip_today from ingest_policy where id=1`).Scan(&v)
	return v, e
}
func (s *Service) AstrBotKey(ctx context.Context) (AstrBotKeyResponse, error) {
	var v AstrBotKeyResponse
	v.HeaderName = AstrBotAPIKeyHeader
	e := s.db.QueryRowContext(ctx, `select astrbot_api_key from sharing_settings where id=1`).Scan(&v.APIKey)
	return v, e
}
func (s *Service) ResetAstrBotKey(ctx context.Context) (AstrBotKeyResponse, error) {
	key, err := generateAPIKey()
	if err != nil {
		return AstrBotKeyResponse{}, err
	}
	_, err = s.db.ExecContext(ctx, `update sharing_settings set astrbot_api_key=? where id=1`, key)
	return AstrBotKeyResponse{APIKey: key, HeaderName: AstrBotAPIKeyHeader}, err
}
func (s *Service) ShareTTLMinutes(ctx context.Context) int {
	p, e := s.Sharing(ctx)
	if e != nil {
		return 60
	}
	return p.ShareTTLMinutes
}
func (s *Service) LogQueryShareTTLMinutes(ctx context.Context) int {
	p, e := s.Sharing(ctx)
	if e != nil {
		return 60
	}
	return p.LogQueryShareTTLMinutes
}
func (s *Service) GetSyncConfig(ctx context.Context) (SyncConfigResponse, error) {
	smb, e := s.SMB(ctx)
	if e != nil {
		return SyncConfigResponse{}, e
	}
	plans, e := s.Schedules(ctx)
	if e != nil {
		return SyncConfigResponse{}, e
	}
	share, e := s.Sharing(ctx)
	if e != nil {
		return SyncConfigResponse{}, e
	}
	return SyncConfigResponse{SMBHost: smb.Host, SMBPort: smb.Port, SMBDomain: smb.Domain, SMBUsername: smb.Username, SMBPassword: smb.Password, SMBShare: smb.Share, Sources: smb.Sources, AutoTasks: plans.Tasks, SkipToday: plans.SkipToday, ShareTTLMinutes: share.ShareTTLMinutes, LogQueryShareTTLMinutes: share.LogQueryShareTTLMinutes}, nil
}

// Compatibility endpoint delegates to the same validation and repository writes.
func (s *Service) SaveSyncConfig(ctx context.Context, p SyncConfigResponse) (SyncConfigResponse, error) {
	smb, e := s.SMB(ctx)
	if e != nil {
		return p, e
	}
	smb.Host = p.SMBHost
	smb.Port = defaultPort(p.SMBPort)
	smb.Domain = p.SMBDomain
	smb.Username = p.SMBUsername
	smb.Password = p.SMBPassword
	smb.Share = p.SMBShare
	if len(p.Sources) > 0 {
		smb.Sources = p.Sources
	}
	plans, e := s.Schedules(ctx)
	if e != nil {
		return p, e
	}
	plans.SkipToday = p.SkipToday
	if len(p.AutoTasks) > 0 {
		plans.Tasks = p.AutoTasks
	}
	share := SharingSettings{p.ShareTTLMinutes, p.LogQueryShareTTLMinutes}
	if e = validateSMB(&smb); e != nil {
		return p, e
	}
	if e = validatePlans(plans); e != nil {
		return p, e
	}
	if e = validateSharing(share); e != nil {
		return p, e
	}
	e = s.db.Write(ctx, func(tx *storage.Tx) error {
		if e := saveSMBTx(ctx, tx, smb); e != nil {
			return e
		}
		if e := savePlansTx(ctx, tx, plans); e != nil {
			return e
		}
		return saveSharingTx(ctx, tx, share)
	})
	if e != nil {
		return p, e
	}
	return s.GetSyncConfig(ctx)
}
func NormalizeShareTTL(minutes int) int {
	if minutes <= 0 {
		return 60
	}
	if minutes < 5 {
		return 5
	}
	if minutes > 10080 {
		return 10080
	}
	return minutes
}

func generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "psk_" + base64.RawURLEncoding.EncodeToString(bytes), nil
}

func defaultPort(port int) int {
	if port <= 0 {
		return 445
	}
	return port
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func nullBool(value sql.NullBool, fallback bool) bool {
	if value.Valid {
		return value.Bool
	}
	return fallback
}
