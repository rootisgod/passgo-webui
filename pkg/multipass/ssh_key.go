package multipass

import (
	"os"
	"path/filepath"
	"runtime"
)

// FindMultipassSSHKey returns the path to the multipass SSH private key
// if it exists, or empty string if not found.
func FindMultipassSSHKey() string {
	candidates := multipassSSHKeyPaths()
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func multipassSSHKeyPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/var/root/Library/Application Support/multipassd/ssh-keys/id_rsa",
		}
	case "linux":
		return []string{
			"/var/snap/multipass/common/data/multipassd/ssh-keys/id_rsa",
			"/var/lib/multipass/ssh-keys/id_rsa",
		}
	case "windows":
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		return []string{
			filepath.Join(programData, "Multipass", "data", "ssh-keys", "id_rsa"),
		}
	default:
		return nil
	}
}
