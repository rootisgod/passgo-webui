package multipass

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScanCloudInitTemplates finds YAML files with "#cloud-config" header in the given directories.
func (c *Client) ScanCloudInitTemplates(searchDirs []string) ([]TemplateOption, error) {
	seen := make(map[string]struct{})
	var options []TemplateOption

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			c.logger.Debug("skip cloud-init dir", "dir", dir, "err", err)
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			lower := strings.ToLower(name)
			if !strings.HasSuffix(lower, ".yml") && !strings.HasSuffix(lower, ".yaml") {
				continue
			}

			path := filepath.Join(dir, name)
			absPath, err := filepath.Abs(path)
			if err != nil {
				continue
			}
			if _, ok := seen[absPath]; ok {
				continue
			}
			if !hasCloudConfigHeader(absPath) {
				continue
			}
			seen[absPath] = struct{}{}
			options = append(options, TemplateOption{Label: name, Path: absPath})
		}
	}

	return options, nil
}

func hasCloudConfigHeader(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()) == "#cloud-config"
	}
	return false
}

// GetAllCloudInitTemplates returns templates from configured directories.
func (c *Client) GetAllCloudInitTemplates(configuredDirs []string) ([]TemplateOption, error) {
	var dirs []string
	dirs = append(dirs, configuredDirs...)

	if exePath, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exePath))
	}
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, cwd)
	}

	// Deduplicate
	seen := make(map[string]struct{})
	var unique []string
	for _, d := range dirs {
		if d == "" {
			continue
		}
		abs, err := filepath.Abs(d)
		if err != nil {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		unique = append(unique, abs)
	}

	return c.ScanCloudInitTemplates(unique)
}

// CloneRepoAndScanYAMLs shallow-clones a repo and returns cloud-init templates found.
func (c *Client) CloneRepoAndScanYAMLs(repoURL string) ([]TemplateOption, string, error) {
	if repoURL == "" {
		return nil, "", fmt.Errorf("empty repo URL")
	}

	tmpDir, err := os.MkdirTemp("", "passgo-web-cloudinit-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temp dir: %w", err)
	}

	if err := cloneRepo(repoURL, tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return nil, "", err
	}

	var options []TemplateOption
	filepath.WalkDir(tmpDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		lower := strings.ToLower(d.Name())
		if !strings.HasSuffix(lower, ".yml") && !strings.HasSuffix(lower, ".yaml") {
			return nil
		}
		rel, _ := filepath.Rel(tmpDir, path)
		options = append(options, TemplateOption{Label: "repo/" + rel, Path: path})
		return nil
	})

	return options, tmpDir, nil
}

func cloneRepo(repoURL, dest string) error {
	cmd := newGitCommand("clone", "--depth", "1", repoURL, dest)
	return cmd.Run()
}

// CleanupTempDirs removes temporary directories.
func CleanupTempDirs(dirs []string) {
	for _, d := range dirs {
		if d != "" {
			os.RemoveAll(d)
		}
	}
}

// sanitizeTemplateName validates a template filename and returns the safe absolute path within baseDir.
func sanitizeTemplateName(baseDir, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("template name is required")
	}
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid template name")
	}
	matched, err := filepath.Match("*.[yY][aA][mM][lL]", name)
	if err != nil || !matched {
		matched2, _ := filepath.Match("*.[yY][mM][lL]", name)
		if !matched2 {
			return "", fmt.Errorf("template name must end in .yml or .yaml")
		}
	}
	// Extra check: only allow safe characters
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
			return "", fmt.Errorf("invalid character in template name: %c", r)
		}
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve base dir: %w", err)
	}
	absPath := filepath.Join(absBase, name)
	// Verify the resolved path is within baseDir
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid template name")
	}
	return absPath, nil
}

// ReadCloudInitTemplate reads the content of a template file by name from baseDir.
func ReadCloudInitTemplate(baseDir, name string) (string, error) {
	path, err := sanitizeTemplateName(baseDir, name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read template: %w", err)
	}
	return string(data), nil
}

// WriteCloudInitTemplate writes content to a template file in baseDir. Creates the directory if needed.
func WriteCloudInitTemplate(baseDir, name, content string) error {
	path, err := sanitizeTemplateName(baseDir, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("create cloud-init dir: %w", err)
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// DeleteCloudInitTemplate removes a template file from baseDir.
func DeleteCloudInitTemplate(baseDir, name string) error {
	path, err := sanitizeTemplateName(baseDir, name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("template not found")
	}
	return os.Remove(path)
}
