package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// handleListHostFiles lists directory entries on the machine running PassGo.
// Used by the host-side "Browse…" picker when adding a mount. The endpoint is
// auth-gated; for homelab deployments (single user) that is an acceptable
// trust boundary, mirroring what an interactive user could see on the host
// anyway.
//
// An empty or missing `path` query defaults to $HOME on Unix or USERPROFILE on
// Windows, falling back to / (or the drive root) if neither is set.
func (s *Server) handleListHostFiles(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("path")
	dirPath := resolveHostPath(raw)

	// Basic guard — reject obvious traversal tricks. filepath.Clean does the
	// real canonicalisation; this just makes the error case clearer in the UI.
	if strings.Contains(raw, "..") {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	dirPath = filepath.Clean(dirPath)

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if os.IsPermission(err) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]fileEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			// Skip entries we can't stat (symlinks to missing targets, etc.).
			continue
		}
		out = append(out, fileEntry{
			Name:        e.Name(),
			Size:        fmt.Sprintf("%d", info.Size()),
			Permissions: info.Mode().String(),
			Modified:    info.ModTime().Format("Jan _2 15:04"),
			IsDir:       e.IsDir(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	writeJSON(w, http.StatusOK, out)
}

// resolveHostPath picks a sensible starting directory when the caller sends an
// empty path.
func resolveHostPath(p string) string {
	if p != "" {
		return p
	}
	return defaultHostHome()
}

// defaultHostHome returns the preferred starting directory for the host
// browser — $HOME if available, otherwise the filesystem root.
func defaultHostHome() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	if runtime.GOOS == "windows" {
		return "C:\\"
	}
	return "/"
}

// handleHostHome returns the default starting path for the host file browser
// so the frontend can use it as a true initial path instead of leaving
// currentPath empty (which breaks joinPath when the user navigates into a
// subfolder).
func (s *Server) handleHostHome(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"path": defaultHostHome()})
}
