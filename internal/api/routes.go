package api

import (
	"embed"
	"log/slog"
	"net/http"
	"time"

	"github.com/rootisgod/passgo-web/internal/auth"
	"github.com/rootisgod/passgo-web/internal/config"
	"github.com/rootisgod/passgo-web/pkg/multipass"
)

type Server struct {
	mp                 *multipass.Client
	cfg                *config.Config
	logger             *slog.Logger
	sessions           *auth.SessionStore
	noAuth             bool
	version            string
	buildTime          string
	gitCommit          string
	builtinTemplatesFS embed.FS
	launches           *launchTracker
}

func NewServer(mp *multipass.Client, cfg *config.Config, logger *slog.Logger, noAuth bool, version, buildTime, gitCommit string, builtinTemplatesFS embed.FS) *Server {
	return &Server{
		mp:                 mp,
		cfg:                cfg,
		logger:             logger,
		sessions:           auth.NewSessionStore(24 * time.Hour),
		noAuth:             noAuth,
		version:            version,
		buildTime:          buildTime,
		gitCommit:          gitCommit,
		builtinTemplatesFS: builtinTemplatesFS,
		launches:           newLaunchTracker(),
	}
}

func (s *Server) Handler(staticFS http.Handler) http.Handler {
	mux := http.NewServeMux()

	// Auth endpoints (no auth required)
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/v1/version", s.handleVersion)

	// Protected API routes
	api := http.NewServeMux()

	// VMs
	api.HandleFunc("GET /api/v1/vms", s.handleListVMs)
	api.HandleFunc("GET /api/v1/vms/{name}", s.handleGetVM)
	api.HandleFunc("POST /api/v1/vms", s.handleCreateVM)
	api.HandleFunc("POST /api/v1/vms/{name}/start", s.handleStartVM)
	api.HandleFunc("POST /api/v1/vms/{name}/stop", s.handleStopVM)
	api.HandleFunc("POST /api/v1/vms/{name}/suspend", s.handleSuspendVM)
	api.HandleFunc("DELETE /api/v1/vms/{name}", s.handleDeleteVM)
	api.HandleFunc("POST /api/v1/vms/{name}/recover", s.handleRecoverVM)
	api.HandleFunc("POST /api/v1/vms/start-all", s.handleStartAll)
	api.HandleFunc("POST /api/v1/vms/stop-all", s.handleStopAll)
	api.HandleFunc("POST /api/v1/vms/purge", s.handlePurge)
	api.HandleFunc("POST /api/v1/vms/{name}/exec", s.handleExecInVM)
	api.HandleFunc("GET /api/v1/launches", s.handleListLaunches)
	api.HandleFunc("DELETE /api/v1/launches/{name}", s.handleDismissLaunch)
	api.HandleFunc("GET /api/v1/vms/{name}/cloud-init/status", s.handleCloudInitStatus)

	// Snapshots
	api.HandleFunc("GET /api/v1/vms/{name}/snapshots", s.handleListSnapshots)
	api.HandleFunc("POST /api/v1/vms/{name}/snapshots", s.handleCreateSnapshot)
	api.HandleFunc("POST /api/v1/vms/{name}/snapshots/{snap}/restore", s.handleRestoreSnapshot)
	api.HandleFunc("DELETE /api/v1/vms/{name}/snapshots/{snap}", s.handleDeleteSnapshot)

	// Mounts
	api.HandleFunc("GET /api/v1/vms/{name}/mounts", s.handleListMounts)
	api.HandleFunc("POST /api/v1/vms/{name}/mounts", s.handleAddMount)
	api.HandleFunc("DELETE /api/v1/vms/{name}/mounts", s.handleRemoveMount)

	// System
	api.HandleFunc("GET /api/v1/networks", s.handleListNetworks)
	api.HandleFunc("GET /api/v1/cloud-init/templates", s.handleListCloudInitTemplates)
	api.HandleFunc("GET /api/v1/cloud-init/templates/{name}", s.handleGetCloudInitTemplate)
	api.HandleFunc("POST /api/v1/cloud-init/templates", s.handleCreateCloudInitTemplate)
	api.HandleFunc("PUT /api/v1/cloud-init/templates/{name}", s.handleUpdateCloudInitTemplate)
	api.HandleFunc("DELETE /api/v1/cloud-init/templates/{name}", s.handleDeleteCloudInitTemplate)

	// Shell WebSocket
	api.HandleFunc("/api/v1/vms/{name}/shell", s.handleShell)

	// Wrap protected routes with auth middleware
	mux.Handle("/api/", authMiddleware(s.sessions, s.noAuth, api))

	// Serve static frontend for all non-API routes
	if staticFS != nil {
		mux.Handle("/", staticFS)
	}

	// Apply global middleware
	var handler http.Handler = mux
	handler = corsMiddleware(handler)
	handler = loggingMiddleware(s.logger, handler)

	return handler
}
