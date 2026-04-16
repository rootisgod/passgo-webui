package api

import (
	"embed"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
	"github.com/rootisgod/passgo-web/pkg/multipass"
)

type Server struct {
	mp                 *multipass.Client
	cfg                *config.Config
	configPath         string // path to the loaded config file — all Save() calls must use this, not DefaultConfigPath
	logger             *slog.Logger
	version            string
	buildTime          string
	gitCommit          string
	builtinTemplatesFS embed.FS
	launches           *launchTracker
	sessions           *sessionStore
	ptySessions        *ptyStore
	// cfgMu serialises read/modify/write of Config. Gates every mutation:
	// groups, VM-group assignments, tokens, webhooks, profiles, schedules,
	// LLM config. Held across cfg.Save() on purpose — releasing it before the
	// write would need a deep Clone() of Config to avoid racing map mutations
	// against json.Marshal, and on a homelab the ms-scale save is not a
	// contention point worth that complexity.
	cfgMu              sync.Mutex
	ansibleRunner      ansibleRunner
	scheduler          *scheduler
	loginLimiter       *loginRateLimiter
	apiLimiter         *apiRateLimiter
	eventLog           *EventLog
}

func NewServer(mp *multipass.Client, cfg *config.Config, configPath string, logger *slog.Logger, version, buildTime, gitCommit string, builtinTemplatesFS embed.FS) *Server {
	s := &Server{
		mp:                 mp,
		cfg:                cfg,
		configPath:         configPath,
		logger:             logger,
		version:            version,
		buildTime:          buildTime,
		gitCommit:          gitCommit,
		builtinTemplatesFS: builtinTemplatesFS,
		launches:           newLaunchTracker(),
		sessions:           newSessionStore(24 * time.Hour),
		ptySessions:        newPtyStore(logger),
		loginLimiter:       newLoginRateLimiter(5, time.Minute),
		apiLimiter:         newAPIRateLimiter(30, time.Minute, cfg.TrustProxy),
	}
	// Wire up ansible queue: when a queued run needs to start, use the server's startPlaybookRun
	s.ansibleRunner.startFunc = s.startPlaybookRun
	s.scheduler = newScheduler(s)
	s.scheduler.start()

	// Event log for audit trail — derive path from the configured config location
	eventsPath := filepath.Join(filepath.Dir(configPath), "events.jsonl")
	el, err := NewEventLog(eventsPath, logger)
	if err != nil {
		logger.Error("failed to open event log", "err", err)
	}
	s.eventLog = el
	if el != nil {
		el.SetDispatcher(s)
	}
	s.ansibleRunner.eventLog = el

	return s
}

// Shutdown cleans up server resources including persistent PTY sessions.
func (s *Server) Shutdown() {
	s.scheduler.stop()
	s.ptySessions.shutdown()
	s.sessions.Shutdown()
	s.loginLimiter.Shutdown()
	s.apiLimiter.Shutdown()
	if s.eventLog != nil {
		s.eventLog.Close()
	}
}

// rateLimited wraps a handler with the API rate limiter.
func (s *Server) rateLimited(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := s.apiLimiter.remoteIP(r)
		if !s.apiLimiter.allow(ip) {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded, try again later")
			return
		}
		handler(w, r)
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
	mux.HandleFunc("POST /api/v1/vms", s.rateLimited(s.handleCreateVM))
	mux.HandleFunc("POST /api/v1/vms/{name}/start", s.handleStartVM)
	mux.HandleFunc("POST /api/v1/vms/{name}/stop", s.handleStopVM)
	mux.HandleFunc("POST /api/v1/vms/{name}/suspend", s.handleSuspendVM)
	mux.HandleFunc("DELETE /api/v1/vms/{name}", s.handleDeleteVM)
	mux.HandleFunc("POST /api/v1/vms/{name}/recover", s.handleRecoverVM)
	mux.HandleFunc("POST /api/v1/vms/start-all", s.handleStartAll)
	mux.HandleFunc("POST /api/v1/vms/stop-all", s.handleStopAll)
	mux.HandleFunc("POST /api/v1/vms/purge", s.handlePurge)
	mux.HandleFunc("POST /api/v1/vms/{name}/clone", s.handleCloneVM)
	mux.HandleFunc("POST /api/v1/vms/{name}/exec", s.handleExecInVM)
	mux.HandleFunc("GET /api/v1/launches", s.handleListLaunches)
	mux.HandleFunc("DELETE /api/v1/launches/{name}", s.handleDismissLaunch)
	mux.HandleFunc("GET /api/v1/vms/{name}/config", s.handleGetVMConfig)
	mux.HandleFunc("PUT /api/v1/vms/{name}/config", s.handleResizeVM)
	mux.HandleFunc("GET /api/v1/vms/{name}/cloud-init/status", s.handleCloudInitStatus)

	// File transfer
	mux.HandleFunc("GET /api/v1/vms/{name}/files", s.handleDownloadFile)
	mux.HandleFunc("POST /api/v1/vms/{name}/files", s.handleUploadFile)
	mux.HandleFunc("GET /api/v1/vms/{name}/files/ls", s.handleListFiles)

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
	mux.HandleFunc("GET /api/v1/host/resources", s.handleHostResources)
	mux.HandleFunc("GET /api/v1/config/vm-defaults", s.handleGetVMDefaults)
	mux.HandleFunc("PUT /api/v1/config/vm-defaults", s.handleUpdateVMDefaults)
	mux.HandleFunc("GET /api/v1/config/export", s.handleExportConfig)
	mux.HandleFunc("POST /api/v1/config/import", s.handleImportConfig)
	mux.HandleFunc("GET /api/v1/networks", s.handleListNetworks)
	mux.HandleFunc("GET /api/v1/cloud-init/templates", s.handleListCloudInitTemplates)
	mux.HandleFunc("GET /api/v1/cloud-init/templates/{name}", s.handleGetCloudInitTemplate)
	mux.HandleFunc("POST /api/v1/cloud-init/templates", s.handleCreateCloudInitTemplate)
	mux.HandleFunc("PUT /api/v1/cloud-init/templates/{name}", s.handleUpdateCloudInitTemplate)
	mux.HandleFunc("DELETE /api/v1/cloud-init/templates/{name}", s.handleDeleteCloudInitTemplate)

	// VM Groups
	mux.HandleFunc("GET /api/v1/groups", s.handleListGroups)
	mux.HandleFunc("POST /api/v1/groups", s.handleCreateGroup)
	mux.HandleFunc("PUT /api/v1/groups/assign", s.handleAssignVmGroup)
	mux.HandleFunc("PUT /api/v1/groups/reorder", s.handleReorderGroups)
	mux.HandleFunc("PUT /api/v1/groups/{name}", s.handleRenameGroup)
	mux.HandleFunc("DELETE /api/v1/groups/{name}", s.handleDeleteGroup)

	// Profiles
	mux.HandleFunc("GET /api/v1/profiles", s.handleListProfiles)
	mux.HandleFunc("POST /api/v1/profiles", s.handleCreateProfile)
	mux.HandleFunc("PUT /api/v1/profiles/{id}", s.handleUpdateProfile)
	mux.HandleFunc("DELETE /api/v1/profiles/{id}", s.handleDeleteProfile)

	// Schedules
	mux.HandleFunc("GET /api/v1/schedules", s.handleListSchedules)
	mux.HandleFunc("POST /api/v1/schedules", s.handleCreateSchedule)
	mux.HandleFunc("PUT /api/v1/schedules/{id}", s.handleUpdateSchedule)
	mux.HandleFunc("DELETE /api/v1/schedules/{id}", s.handleDeleteSchedule)
	mux.HandleFunc("POST /api/v1/schedules/{id}/run", s.handleRunScheduleNow)
	mux.HandleFunc("GET /api/v1/schedules/history", s.handleScheduleHistory)

	// Ansible
	mux.HandleFunc("GET /api/v1/ansible/inventory", s.handleAnsibleInventory)
	mux.HandleFunc("GET /api/v1/ansible/status", s.handleAnsibleStatus)
	mux.HandleFunc("GET /api/v1/ansible/playbooks", s.handleListPlaybooks)
	mux.HandleFunc("GET /api/v1/ansible/playbooks/{name}", s.handleGetPlaybook)
	mux.HandleFunc("POST /api/v1/ansible/playbooks", s.handleCreatePlaybook)
	mux.HandleFunc("PUT /api/v1/ansible/playbooks/{name}", s.handleUpdatePlaybook)
	mux.HandleFunc("DELETE /api/v1/ansible/playbooks/{name}", s.handleDeletePlaybook)
	mux.HandleFunc("POST /api/v1/ansible/run", s.handleRunPlaybook)
	mux.HandleFunc("GET /api/v1/ansible/run/status", s.handleAnsibleRunStatus)
	mux.HandleFunc("GET /api/v1/ansible/run/output", s.handleAnsibleRunOutput)
	mux.HandleFunc("DELETE /api/v1/ansible/run", s.handleCancelAnsibleRun)
	mux.HandleFunc("POST /api/v1/ansible/run/clear", s.handleClearAnsibleRun)
	mux.HandleFunc("GET /api/v1/ansible/run/queue", s.handleAnsibleRunQueue)
	mux.HandleFunc("DELETE /api/v1/ansible/run/queue", s.handleClearAnsibleRunQueue)

	// API Tokens
	mux.HandleFunc("GET /api/v1/tokens", s.handleListTokens)
	mux.HandleFunc("POST /api/v1/tokens", s.handleCreateToken)
	mux.HandleFunc("DELETE /api/v1/tokens/{id}", s.handleDeleteToken)

	// Webhooks
	mux.HandleFunc("GET /api/v1/webhooks", s.handleListWebhooks)
	mux.HandleFunc("POST /api/v1/webhooks", s.handleCreateWebhook)
	mux.HandleFunc("PUT /api/v1/webhooks/{id}", s.handleUpdateWebhook)
	mux.HandleFunc("DELETE /api/v1/webhooks/{id}", s.handleDeleteWebhook)
	mux.HandleFunc("POST /api/v1/webhooks/{id}/test", s.handleTestWebhook)

	// Event log
	mux.HandleFunc("GET /api/v1/events", s.handleListEvents)

	// Chat / LLM
	mux.HandleFunc("POST /api/v1/chat", s.rateLimited(s.handleChat))
	mux.HandleFunc("GET /api/v1/chat/config", s.handleGetChatConfig)
	mux.HandleFunc("PUT /api/v1/chat/config", s.handleUpdateChatConfig)
	mux.HandleFunc("GET /api/v1/chat/models", s.handleListModels)

	// Shell sessions
	mux.HandleFunc("POST /api/v1/vms/{name}/shell/sessions", s.handleCreateShellSession)
	mux.HandleFunc("GET /api/v1/vms/{name}/shell/sessions", s.handleListShellSessions)
	mux.HandleFunc("DELETE /api/v1/vms/{name}/shell/sessions/{sessionId}", s.handleDeleteShellSession)
	mux.HandleFunc("/api/v1/vms/{name}/shell/{sessionId}", s.handleShell)

	// Serve static frontend for all non-API routes
	if staticFS != nil {
		mux.Handle("/", staticFS)
	}

	// Apply global middleware (outermost first)
	var handler http.Handler = mux
	handler = authMiddleware(s.sessions, s.cfg, handler)
	handler = bodySizeLimitMiddleware(handler)
	handler = corsMiddleware(handler)
	handler = securityHeadersMiddleware(handler)
	handler = loggingMiddleware(s.logger, handler)

	return handler
}
