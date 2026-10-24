package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"os"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/configmigration"
	"strconv"
	"time"
	_ "time/tzdata"
)

func main() {
	address := flag.String("mysql-address", "mysql:3306", "legacy MySQL address")
	output := flag.String("output", "/data/database/player.db", "new SQLite path; must not exist")
	flag.Parse()
	cfg := config.Load()
	for _, p := range []struct {
		k   string
		dst *int
	}{{"PLAYER_LOGS_LIVE_CACHE_SECONDS", &cfg.LiveLogSnapshotTTLSeconds}, {"PLAYER_LOGS_IMPORTER_MAX_FILES_PER_RUN", &cfg.MaxFilesPerRun}, {"PLAYER_LOGS_XRAY_ANALYSIS_RETAINED_JOBS", &cfg.XrayRetainedJobs}} {
		if v := os.Getenv(p.k); v != "" {
			n, e := strconv.Atoi(v)
			if e != nil {
				fmt.Fprintln(os.Stderr, "invalid legacy tuning:", p.k)
				os.Exit(1)
			}
			*p.dst = n
		}
	}
	c := mysql.NewConfig()
	c.User = os.Getenv("MYSQL_USER")
	c.Passwd = os.Getenv("MYSQL_PASSWORD")
	c.DBName = os.Getenv("MYSQL_DATABASE")
	c.Net = "tcp"
	c.Addr = *address
	c.ParseTime = true
	c.Loc = cfg.Location
	c.Timeout = 10 * time.Second
	c.ReadTimeout = 30 * time.Second
	c.Params = map[string]string{"charset": "utf8mb4"}
	if c.User == "" || c.Passwd == "" || c.DBName == "" {
		fmt.Fprintln(os.Stderr, "MYSQL_USER, MYSQL_PASSWORD and MYSQL_DATABASE are required for the migration tool")
		os.Exit(1)
	}
	db, e := sql.Open("mysql", c.FormatDSN())
	if e != nil {
		fmt.Fprintln(os.Stderr, "cannot initialize source connection")
		os.Exit(1)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if e = configmigration.Migrate(ctx, db, *output, cfg); e != nil {
		fmt.Fprintln(os.Stderr, "migration failed:", e)
		os.Exit(1)
	}
	fmt.Println("Configuration migrated; run rebuild before starting the application.")
}
