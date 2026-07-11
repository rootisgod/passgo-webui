package api

import (
	"fmt"
	"os"
)

func writeTempCloudInit(content []byte) (string, error) {
	file, err := os.CreateTemp("", "passgo-cloudinit-*.yml")
	if err != nil {
		return "", err
	}
	path := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(path)
	}
	if err := file.Chmod(0600); err != nil {
		cleanup()
		return "", fmt.Errorf("chmod temp cloud-init: %w", err)
	}
	if _, err := file.Write(content); err != nil {
		cleanup()
		return "", fmt.Errorf("write temp cloud-init: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close temp cloud-init: %w", err)
	}
	return path, nil
}
