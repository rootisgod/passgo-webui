package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
	"github.com/rootisgod/passgo-web/pkg/multipass"
)

type configBundle struct {
	Version            int               `json:"version"`
	ExportedAt         string            `json:"exported_at"`
	Config             *configExport     `json:"config"`
	CloudInitTemplates map[string]string `json:"cloud_init_templates"`
	Playbooks          map[string]string `json:"playbooks"`
}

type configExport struct {
	Groups     []string            `json:"groups,omitempty"`
	VMGroups   map[string]string   `json:"vm_groups,omitempty"`
	VMDefaults *config.VMDefaults  `json:"vm_defaults,omitempty"`
	LLM        *llmConfigExport    `json:"llm,omitempty"`
	Profiles   []config.Profile    `json:"profiles,omitempty"`
}

type llmConfigExport struct {
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

func (s *Server) handleExportConfig(w http.ResponseWriter, r *http.Request) {
	s.groupMu.Lock()
	export := &configExport{
		Groups:     s.cfg.Groups,
		VMGroups:   s.cfg.VMGroups,
		VMDefaults: s.cfg.VMDefaults,
		Profiles:   s.cfg.GetProfiles(),
	}
	if s.cfg.LLM != nil {
		export.LLM = &llmConfigExport{
			BaseURL:  s.cfg.LLM.BaseURL,
			Model:    s.cfg.LLM.Model,
			ReadOnly: s.cfg.LLM.ReadOnly,
		}
	}
	cloudInitDir := s.cfg.CloudInitDir
	playbooksDir := s.cfg.PlaybooksDir
	s.groupMu.Unlock()

	// Read user cloud-init templates
	templates := make(map[string]string)
	if entries, err := os.ReadDir(cloudInitDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			lower := strings.ToLower(e.Name())
			if !strings.HasSuffix(lower, ".yml") && !strings.HasSuffix(lower, ".yaml") {
				continue
			}
			content, err := multipass.ReadCloudInitTemplate(cloudInitDir, e.Name())
			if err != nil {
				s.logger.Warn("export: skip cloud-init template", "name", e.Name(), "err", err)
				continue
			}
			templates[e.Name()] = content
		}
	}

	// Read playbooks
	playbooks := make(map[string]string)
	names, err := multipass.ListPlaybooks(playbooksDir)
	if err == nil {
		for _, name := range names {
			content, err := multipass.ReadPlaybook(playbooksDir, name)
			if err != nil {
				s.logger.Warn("export: skip playbook", "name", name, "err", err)
				continue
			}
			playbooks[name] = content
		}
	}

	bundle := &configBundle{
		Version:            1,
		ExportedAt:         time.Now().UTC().Format(time.RFC3339),
		Config:             export,
		CloudInitTemplates: templates,
		Playbooks:          playbooks,
	}

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "marshal config: "+err.Error())
		return
	}

	filename := fmt.Sprintf("passgo-config-%s.json", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *Server) handleImportConfig(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB

	var bundle configBundle
	if err := json.NewDecoder(r.Body).Decode(&bundle); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if bundle.Version != 1 {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported config version: %d", bundle.Version))
		return
	}

	s.groupMu.Lock()
	if bundle.Config != nil {
		if bundle.Config.Groups != nil {
			s.cfg.Groups = bundle.Config.Groups
		}
		if bundle.Config.VMGroups != nil {
			s.cfg.VMGroups = bundle.Config.VMGroups
		}
		if bundle.Config.VMDefaults != nil {
			s.cfg.VMDefaults = bundle.Config.VMDefaults
		}
		if bundle.Config.Profiles != nil {
			s.cfg.Profiles = bundle.Config.Profiles
		}
		if bundle.Config.LLM != nil {
			if s.cfg.LLM == nil {
				s.cfg.LLM = &config.LLMConfig{}
			}
			s.cfg.LLM.BaseURL = bundle.Config.LLM.BaseURL
			s.cfg.LLM.Model = bundle.Config.LLM.Model
			s.cfg.LLM.ReadOnly = bundle.Config.LLM.ReadOnly
			// Preserve existing API key
		}
	}
	cloudInitDir := s.cfg.CloudInitDir
	playbooksDir := s.cfg.PlaybooksDir

	if err := s.cfg.Save(config.DefaultConfigPath()); err != nil {
		s.groupMu.Unlock()
		writeError(w, http.StatusInternalServerError, "save config: "+err.Error())
		return
	}
	s.groupMu.Unlock()

	templatesWritten := 0
	for name, content := range bundle.CloudInitTemplates {
		if err := multipass.WriteCloudInitTemplate(cloudInitDir, name, content); err != nil {
			s.logger.Warn("import: skip cloud-init template", "name", name, "err", err)
			continue
		}
		templatesWritten++
	}

	playbooksWritten := 0
	for name, content := range bundle.Playbooks {
		if err := multipass.WritePlaybook(playbooksDir, name, content); err != nil {
			s.logger.Warn("import: skip playbook", "name", name, "err", err)
			continue
		}
		playbooksWritten++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":           "Configuration imported",
		"templates_written": templatesWritten,
		"playbooks_written": playbooksWritten,
	})
}
