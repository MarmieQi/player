package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServerPort            string
	ZoneID                string
	Location              *time.Location
	AllowedOrigins        []string
	AllowedOriginPatterns []string

	DatasourceURL      string
	DatasourceUsername string
	DatasourcePassword string

	AdminUsername     string
	AdminPassword     string
	AdminTokenTTLDays int
	AdminJWTSecret    string
	AdminJWTIssuer    string

	LogArchiveDirectory       string
	MainName                  string
	SubName                   string
	MainDirectory             string
	SubDirectory              string
	MainFileGlob              string
	SubFileGlob               string
	MainEnabled               bool
	SubEnabled                bool
	MaxFilesPerRun            int
	XrayRetainedJobs          int
	XrayTempDirectory         string
	LiveLogSnapshotDirectory  string
	LiveLogSnapshotTTLSeconds int
}

func Load() Config {
	zoneID := env("PLAYER_LOGS_ZONE_ID", "Asia/Shanghai")
	location, err := time.LoadLocation(zoneID)
	if err != nil {
		location = time.Local
	}

	return Config{
		ServerPort:            env("SERVER_PORT", "8080"),
		ZoneID:                zoneID,
		Location:              location,
		AllowedOrigins:        csvEnv("APP_CORS_ALLOWED_ORIGINS", "http://localhost:5173"),
		AllowedOriginPatterns: csvEnv("APP_CORS_ALLOWED_ORIGIN_PATTERNS", "http://localhost:*,http://127.0.0.1:*,http://10.*:*,http://172.*:*,http://192.168.*:*"),

		DatasourceURL:      env("PLAYER_STATS_DATASOURCE_URL", "jdbc:mysql://localhost:3306/player_stats?useUnicode=true&characterEncoding=utf8&useSSL=false&allowPublicKeyRetrieval=true&serverTimezone=Asia/Shanghai&rewriteBatchedStatements=true&createDatabaseIfNotExist=true"),
		DatasourceUsername: env("PLAYER_STATS_DATASOURCE_USERNAME", "root"),
		DatasourcePassword: env("PLAYER_STATS_DATASOURCE_PASSWORD", "mysql"),

		AdminUsername:     env("APP_ADMIN_USERNAME", "admin"),
		AdminPassword:     env("APP_ADMIN_PASSWORD", "admin123456"),
		AdminTokenTTLDays: intEnv("APP_ADMIN_TOKEN_TTL_DAYS", 7),
		AdminJWTSecret:    env("APP_ADMIN_JWT_SECRET", "change-me-player-stats-jwt-secret-change-me"),
		AdminJWTIssuer:    env("APP_ADMIN_JWT_ISSUER", "player-stats"),

		LogArchiveDirectory:       env("PLAYER_LOGS_ARCHIVE_DIRECTORY", "./synced-logs"),
		MainName:                  env("PLAYER_LOGS_MAIN_NAME", "主服"),
		SubName:                   env("PLAYER_LOGS_SUB_NAME", "2服"),
		MainDirectory:             env("PLAYER_LOGS_MAIN_DIRECTORY", "./synced-logs/main"),
		SubDirectory:              env("PLAYER_LOGS_SUB_DIRECTORY", "./synced-logs/sub"),
		MainFileGlob:              env("PLAYER_LOGS_MAIN_FILE_GLOB", "player_actions_*.csv"),
		SubFileGlob:               env("PLAYER_LOGS_SUB_FILE_GLOB", "player_actions_*.csv"),
		MainEnabled:               boolEnv("PLAYER_LOGS_MAIN_ENABLED", true),
		SubEnabled:                boolEnv("PLAYER_LOGS_SUB_ENABLED", true),
		MaxFilesPerRun:            intEnv("PLAYER_LOGS_IMPORTER_MAX_FILES_PER_RUN", 1000),
		XrayRetainedJobs:          intEnv("PLAYER_LOGS_XRAY_ANALYSIS_RETAINED_JOBS", 1),
		XrayTempDirectory:         env("PLAYER_LOGS_XRAY_TEMP_DIRECTORY", filepath.Join(os.TempDir(), "player-stats-xray")),
		LiveLogSnapshotDirectory:  env("PLAYER_LOGS_LIVE_TEMP_DIRECTORY", filepath.Join(os.TempDir(), "player-stats-live-log-query")),
		LiveLogSnapshotTTLSeconds: intEnv("PLAYER_LOGS_LIVE_CACHE_SECONDS", 120),
	}
}

type Source struct {
	ID        string
	Name      string
	Directory string
	FileGlob  string
	Enabled   bool
}

func (c Config) Sources() []Source {
	sources := []Source{
		{ID: "main", Name: c.MainName, Directory: c.MainDirectory, FileGlob: c.MainFileGlob, Enabled: c.MainEnabled},
		{ID: "sub", Name: c.SubName, Directory: c.SubDirectory, FileGlob: c.SubFileGlob, Enabled: c.SubEnabled},
	}
	enabled := make([]Source, 0, len(sources))
	for _, source := range sources {
		if source.Enabled {
			enabled = append(enabled, source)
		}
	}
	return enabled
}

func (c Config) SourceName(serverID string) string {
	if serverID == "" {
		return "合计"
	}
	for _, source := range c.Sources() {
		if source.ID == serverID {
			return source.Name
		}
	}
	return serverID
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func csvEnv(key, fallback string) []string {
	raw := env(key, fallback)
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func intEnv(key string, fallback int) int {
	raw := strings.TrimSpace(env(key, ""))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func boolEnv(key string, fallback bool) bool {
	raw := strings.TrimSpace(env(key, ""))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}
