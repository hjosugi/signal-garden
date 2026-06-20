package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hjosugi/signal-garden/internal/ai"
	"github.com/hjosugi/signal-garden/internal/collector"
	"github.com/hjosugi/signal-garden/internal/config"
	"github.com/hjosugi/signal-garden/internal/feed"
	"github.com/hjosugi/signal-garden/internal/store"
	"github.com/hjosugi/signal-garden/internal/tagger"
	webapp "github.com/hjosugi/signal-garden/internal/web"
)

func main() {
	app, err := config.LoadApp()
	if err != nil {
		fatal("load app config", err)
	}
	logger := newLogger(app.LogJSON)
	slog.SetDefault(logger)

	site, err := config.LoadSite(app.SitePath)
	if err != nil {
		fatal("load site config", err)
	}
	feeds, err := config.LoadFeeds(app.FeedsPath)
	if err != nil {
		fatal("load feeds config", err)
	}
	if err := collector.ValidateFeeds(feeds); err != nil {
		fatal("validate feeds", err)
	}
	dataStore, err := store.OpenJSON(app.DataPath, app.MaxItems)
	if err != nil {
		fatal("open data store", err)
	}

	hub := webapp.NewHub()
	ollama := ai.NewOllama(app.OllamaBaseURL, app.OllamaChatModel, app.OllamaEmbedModel, app.SummaryLanguage, app.RequestTimeout*3)
	col := collector.New(
		feeds,
		feed.NewFetcher(app.RequestTimeout),
		dataStore,
		tagger.New(),
		ollama,
		app.EnableLLMSummary,
		app.EnableEmbeddings,
		app.FeedWorkers,
		logger,
		hub.Publish,
	)
	webServer, err := webapp.NewServer(app, site, feeds, dataStore, col, ollama, hub, logger)
	if err != nil {
		fatal("create web server", err)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if app.InitialRefresh {
		_ = col.RefreshAsync(rootCtx)
	}
	if app.RefreshInterval > 0 {
		go scheduleRefresh(rootCtx, col, app.RefreshInterval, logger)
	}

	httpServer := &http.Server{
		Addr:              app.Addr,
		Handler:           webServer.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("signal-garden listening", "addr", app.Addr, "items", dataStore.Count())
		serverErr <- httpServer.ListenAndServe()
	}()

	select {
	case <-rootCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			fatal("HTTP server", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		_ = httpServer.Close()
	}
}

func scheduleRefresh(ctx context.Context, col *collector.Collector, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := col.RefreshAsync(ctx); err != nil && !errors.Is(err, collector.ErrAlreadyRunning) {
				logger.Warn("scheduled refresh not started", "error", err)
			}
		}
	}
}

func newLogger(json bool) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if json {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func fatal(message string, err error) {
	slog.Error(message, "error", err)
	os.Exit(1)
}
