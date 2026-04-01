package api

import (
	"embed"
	"log/slog"
	"net/http"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
	"github.com/rootisgod/passgo-web/pkg/multipass"
)

type Server struct {
	mp                 *multipass.Client
	cfg                *config.Config
	logger             *slog.Logger
	version            string
	buildTime          string
	gitCommit          string
	builtinTemplatesFS embed.FS
	launches           *launchTracker
	sessions           *sessionStore
}

func NewServer(mp *multipass.Client, cfg *config.Config, logger *slog.Logger, version, buildTime, gitCommit string, builtinTemplatesFS embed.FS) *Server {
	return &Server{
		mp:                 mp,
		cfg:                cfg,
		logger:             logger,
		version:            version,
		buildTime:          buildTime,
		gitCommit:          gitCommit,
		builtinTemplatesFS: builtinTemplatesFS,
		launches:           newLaunchTracker(),
		sessions:           newSessionStore(24 * time.Hour),
	}
}

func (s *Server) Handler(staticFS http.Handler) http.Handler {
	mux := http.NewServeMux()

	// Auth
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)

	// API routes
	mux.HandleFunc("GET /api/v1/version", s.handleVersion)

	// VMs
	mux.HandleFunc("GET /api/v1/vms", s.handleListVMs)
	mux.HandleFunc("GET /api/v1/vms/{name}", s.handleGetVM)
	mux.HandleFunc("POST /api/v1/vms", s.handleCreateVM)
	mux.HandleFunc("POST /api/v1/vms/{name}/start", s.handleStartVM)
	mux.HandleFunc("POST /api/v1/vms/{name}/stop", s.handleStopVM)
	mux.HandleFunc("POST /api/v1/vms/{name}/suspend", s.handleSuspendVM)
	mux.HandleFunc("DELETE /api/v1/vms/{name}", s.handleDeleteVM)
	mux.HandleFunc("POST /api/v1/vms/{name}/recover", s.handleRecoverVM)
	mux.HandleFunc("POST /api/v1/vms/start-all", s.handleStartAll)
	mux.HandleFunc("POST /api/v1/vms/stop-all", s.handleStopAll)
	mux.HandleFunc("POST /api/v1/vms/purge", s.handlePurge)
	mux.HandleFunc("POST /api/v1/vms/{name}/exec", s.handleExecInVM)
	mux.HandleFunc("GET /api/v1/launches", s.handleListLaunches)
	mux.HandleFunc("DELETE /api/v1/launches/{name}", s.handleDismissLaunch)
	mux.HandleFunc("GET /api/v1/vms/{name}/cloud-init/status", s.handleCloudInitStatus)

	// Snapshots
	mux.HandleFunc("GET /api/v1/vms/{name}/snapshots", s.handleListSnapshots)
	mux.HandleFunc("POST /api/v1/vms/{name}/snapshots", s.handleCreateSnapshot)
	mux.HandleFunc("POST /api/v1/vms/{name}/snapshots/{snap}/restore", s.handleRestoreSnapshot)
	mux.HandleFunc("DELETE /api/v1/vms/{name}/snapshots/{snap}", s.handleDeleteSnapshot)

	// Mounts
	mux.HandleFunc("GET /api/v1/vms/{name}/mounts", s.handleListMounts)
	mux.HandleFunc("POST /api/v1/vms/{name}/mounts", s.handleAddMount)
	mux.HandleFunc("DELETE /api/v1/vms/{name}/mounts", s.handleRemoveMount)

	// System
	mux.HandleFunc("GET /api/v1/images", s.handleFindImages)
	mux.HandleFunc("GET /api/v1/networks", s.handleListNetworks)
	mux.HandleFunc("GET /api/v1/cloud-init/templates", s.handleListCloudInitTemplates)
	mux.HandleFunc("GET /api/v1/cloud-init/templates/{name}", s.handleGetCloudInitTemplate)
	mux.HandleFunc("POST /api/v1/cloud-init/templates", s.handleCreateCloudInitTemplate)
	mux.HandleFunc("PUT /api/v1/cloud-init/templates/{name}", s.handleUpdateCloudInitTemplate)
	mux.HandleFunc("DELETE /api/v1/cloud-init/templates/{name}", s.handleDeleteCloudInitTemplate)

	// Shell WebSocket
	mux.HandleFunc("/api/v1/vms/{name}/shell", s.handleShell)

	// Serve static frontend for all non-API routes
	if staticFS != nil {
		mux.Handle("/", staticFS)
	}

	// Apply global middleware
	var handler http.Handler = mux
	handler = authMiddleware(s.sessions, handler)
	handler = corsMiddleware(handler)
	handler = loggingMiddleware(s.logger, handler)

	return handler
}
