package api

import (
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strings"
)

func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	remotePath := r.URL.Query().Get("path")
	if remotePath == "" {
		writeError(w, http.StatusBadRequest, "path query parameter is required")
		return
	}
	if !validateRemotePath(remotePath) {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	filename := sanitizeFilename(path.Base(remotePath))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", "application/octet-stream")

	if err := s.mp.TransferFromVM(name, remotePath, w); err != nil {
		// Headers already sent if partial data was written; log the error
		s.logger.Error("file download failed", "err", err, "vm", name, "path", remotePath)
		return
	}
}

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// 32MB limit
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	destDir := r.FormValue("path")
	if destDir == "" {
		destDir = "/home/ubuntu"
	}
	if !validateRemotePath(destDir) {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	remotePath := destDir + "/" + header.Filename

	if err := s.mp.TransferToVM(name, remotePath, file); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeMessage(w, fmt.Sprintf("uploaded %s to %s", header.Filename, remotePath))
}

type fileEntry struct {
	Name        string `json:"name"`
	Size        string `json:"size"`
	Permissions string `json:"permissions"`
	Modified    string `json:"modified"`
	IsDir       bool   `json:"isDir"`
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	dirPath := r.URL.Query().Get("path")
	if dirPath == "" {
		dirPath = "/home/ubuntu"
	}
	if !validateRemotePath(dirPath) {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	output, err := s.mp.ExecInVM(name, []string{"ls", "-la", dirPath})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	entries := parseLsOutput(output)
	writeJSON(w, http.StatusOK, entries)
}

// parseLsOutput parses `ls -la` output into file entries.
func parseLsOutput(output string) []fileEntry {
	var entries []fileEntry
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		name := strings.Join(fields[8:], " ")
		if name == "." || name == ".." {
			continue
		}

		perms := fields[0]
		isDir := len(perms) > 0 && perms[0] == 'd'
		size := fields[4]
		modified := fields[5] + " " + fields[6] + " " + fields[7]

		entries = append(entries, fileEntry{
			Name:        name,
			Size:        size,
			Permissions: perms,
			Modified:    modified,
			IsDir:       isDir,
		})
	}

	return entries
}

// sanitizeFilename removes characters that could break HTTP headers or enable injection.
var unsafeFilenameChars = regexp.MustCompile(`["\x00-\x1f\x7f]`)

func sanitizeFilename(name string) string {
	return unsafeFilenameChars.ReplaceAllString(name, "_")
}

// validateRemotePath checks that a remote VM path doesn't contain traversal sequences.
func validateRemotePath(p string) bool {
	// Reject path traversal attempts
	if strings.Contains(p, "..") {
		return false
	}
	// Must be absolute
	if !strings.HasPrefix(p, "/") {
		return false
	}
	return true
}
