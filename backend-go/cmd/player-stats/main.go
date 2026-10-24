package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"player-stats-backend-go/internal/auth"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/groupchat"
	"player-stats-backend-go/internal/httpapi"
	"player-stats-backend-go/internal/importer"
	"player-stats-backend-go/internal/settings"
	"player-stats-backend-go/internal/share"
	"player-stats-backend-go/internal/stats"
	"player-stats-backend-go/internal/storage"
)

func main() {
	cfg := config.Load()
	release, lockErr := storage.ProcessLock(cfg.DatabasePath)
	if lockErr != nil {
		fmt.Fprintln(os.Stderr, lockErr)
		os.Exit(1)
	}
	defer release()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer startupCancel()
	db, err := storage.Open(startupCtx, cfg)
	if err != nil {
		logger.Error("SQLite connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := storage.Migrate(startupCtx, db); err != nil {
		logger.Error("schema migration failed", "error", err)
		os.Exit(1)
	}
	if err := storage.EnsureDefaults(startupCtx, db, cfg); err != nil {
		logger.Error("default config initialization failed", "error", err)
		os.Exit(1)
	}
	cfg, err = settings.InitializeRuntime(startupCtx, db, cfg)
	if err != nil {
		logger.Error("runtime configuration failed", "error", err)
		os.Exit(1)
	}
	logger.Info("runtime parameters loaded", "liveCacheSeconds", cfg.LiveLogSnapshotTTLSeconds, "maxFilesPerRun", cfg.MaxFilesPerRun, "xrayRetainedJobs", cfg.XrayRetainedJobs)
	instanceID, err := storage.InstanceID(startupCtx, db)
	if err != nil {
		logger.Error("database identity failed", "error", err)
		return
	}
	cfg.AdminJWTIssuer = cfg.AdminJWTIssuer + ":" + instanceID
	if len(os.Args) > 1 {
		if os.Args[1] != "rebuild" {
			logger.Error("unknown command")
			os.Exit(2)
		}
		rebuildCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		service := importer.NewService(db, cfg, settings.NewService(db, cfg))
		defer service.CloseLiveLogSnapshots()
		if err := service.Rebuild(rebuildCtx, nil); err != nil {
			logger.Error("rebuild failed; progress retained", "error", err)
			os.Exit(1)
		}
		logger.Info("rebuild complete")
		return
	}
	needsRebuild, err := storage.RequiresRebuild(startupCtx, db)
	if err != nil || needsRebuild {
		logger.Error("database requires offline rebuild before server startup", "error", err)
		os.Exit(1)
	}
	authService := auth.NewService(db, cfg)
	if err := authService.EnsureAdmin(startupCtx); err != nil {
		logger.Error("admin initialization failed", "error", err)
		os.Exit(1)
	}
	settingsService := settings.NewService(db, cfg)
	statsService := stats.NewService(db, cfg)
	importService := importer.NewService(db, cfg, settingsService)
	if err := importService.RecoverTaskRuns(startupCtx); err != nil {
		logger.Error("task recovery failed", "error", err)
		os.Exit(1)
	}
	importJobs := importer.NewJobService(importService)
	defer importJobs.Close()
	logQueryService := importer.NewLogQueryService(importService)
	xrayService := importer.NewXrayAnalysisService(importService)
	shareService := share.NewService(db, cfg, settingsService, statsService, xrayService, logQueryService)
	chatCopyService := groupchat.NewCopyService(db, cfg, importService)
	defer chatCopyService.Close()
	chatQueryService := groupchat.NewQueryService(chatCopyService, cfg.Location)
	defer chatQueryService.Close()
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	go runShareTokenCleanup(cleanupCtx, shareService, logger)
	go importService.RunLiveLogSnapshotCleanup(cleanupCtx)
	go groupchat.NewScheduler(chatCopyService, logger, cfg.Location).Run(cleanupCtx)
	go chatQueryService.RunCleanup(cleanupCtx)
	defer cleanupCancel()
	autoScheduler := importer.NewAutoTaskScheduler(settingsService, importService, logger, cfg.Location)
	autoScheduler.Start()
	defer autoScheduler.Stop()

	app := httpapi.NewServer(cfg, logger, authService, settingsService, shareService, importService, importJobs, logQueryService, xrayService, statsService, chatCopyService, chatQueryService)
	server := &http.Server{
		Addr:              net.JoinHostPort(cfg.ServerHost, cfg.ServerPort),
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("go backend listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	chatQueryService.Close()
	chatCopyService.Close()
	importJobs.Close()
	cleanupCancel()
	autoScheduler.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
	importService.CloseLiveLogSnapshots()
}

func runShareTokenCleanup(ctx context.Context, service *share.Service, logger *slog.Logger) {
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := service.CleanupExpiredTokens(cleanupCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Warn("expired share token cleanup failed", "error", err)
		}
	}
	cleanup()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}
