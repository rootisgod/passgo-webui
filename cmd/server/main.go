package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rootisgod/passgo-web/internal/api"
	"github.com/rootisgod/passgo-web/internal/config"
	"github.com/rootisgod/passgo-web/pkg/multipass"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

//go:embed cloud-init/*.yml
var builtinTemplatesFS embed.FS

func main() {
	var (
		port       int
		configPath string
		showVer    bool
		username   string
		password   string
	)

	flag.IntVar(&port, "port", 0, "Listen port (overrides config)")
	flag.StringVar(&configPath, "config", config.DefaultConfigPath(), "Config file path")
	flag.BoolVar(&showVer, "version", false, "Print version and exit")
	flag.StringVar(&username, "username", "", "Login username (overrides config)")
	flag.StringVar(&password, "password", "", "Login password (overrides config)")
	flag.Parse()

	if showVer {
		fmt.Printf("PassGo Web %s (built %s, commit %s)\n", Version, BuildTime, GitCommit)
		os.Exit(0)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Load or create config
	cfg, err := config.Load(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Error("failed to load config", "path", configPath, "err", err)
			os.Exit(1)
		}
		logger.Info("no config found, creating default", "path", configPath)
		cfg, err = config.CreateDefault(configPath)
		if err != nil {
			logger.Error("failed to create config", "err", err)
			os.Exit(1)
		}
		fmt.Printf("Config: %s\n", configPath)
	}

	// Auto-migrate plaintext passwords to bcrypt (plaintext login no longer supported)
	if migrated, err := config.MigratePassword(cfg, configPath); err != nil {
		logger.Warn("failed to migrate password to bcrypt", "err", err)
	} else if migrated {
		logger.Info("migrated plaintext password to bcrypt hash")
	}

	// Override from flags
	if port > 0 {
		cfg.Listen = fmt.Sprintf(":%d", port)
	}
	if username != "" {
		cfg.Username = username
	}
	if password != "" {
		hashed, err := config.HashPassword(password)
		if err != nil {
			logger.Error("failed to hash password", "err", err)
			os.Exit(1)
		}
		cfg.Password = hashed
	}

	// Create multipass client
	mp := multipass.NewClient(logger)

	// Set up static file serving from embedded frontend
	var staticFS http.Handler
	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		logger.Warn("no embedded frontend found, API-only mode", "err", err)
	} else {
		staticFS = spaHandler(http.FileServerFS(distFS), distFS)
	}

	// Create and start server
	srv := api.NewServer(mp, cfg, configPath, logger, Version, BuildTime, GitCommit, builtinTemplatesFS)
	handler := srv.Handler(staticFS)

	listen := cfg.Listen
	if !strings.Contains(listen, ":") {
		listen = ":" + listen
	}

	fmt.Printf("PassGo Web %s\n", Version)
	fmt.Printf("Config: %s\n", configPath)
	fmt.Printf("Listening on http://0.0.0.0%s\n", listen)

	// Explicit server with timeouts. No WriteTimeout: shell WebSockets and
	// LLM chat SSE are long-lived writes; per-request timeouts handle those.
	httpSrv := &http.Server{
		Addr:              listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown: drain in-flight requests before killing PTYs, scheduler, eventlog.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			logger.Warn("http shutdown error", "err", err)
		}
		srv.Shutdown()
	}()

	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "err", err)
		os.Exit(1)
	}
}

// spaHandler serves the SPA — returns index.html for any path that doesn't match a real file.
func spaHandler(fileServer http.Handler, fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Strip leading slash for fs.Stat
		cleanPath := strings.TrimPrefix(path, "/")
		if _, err := fs.Stat(fsys, cleanPath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found — serve index.html for SPA routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
