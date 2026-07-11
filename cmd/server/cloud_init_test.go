package main

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDesktopCloudInitKeepsVNCOnGuestLoopback(t *testing.T) {
	data, err := os.ReadFile("cloud-init/ubuntu-desktop-novnc.yml")
	if err != nil {
		t.Fatal(err)
	}
	var document any
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("invalid cloud-init YAML: %v", err)
	}

	content := string(data)
	for _, required := range []string{"-localhost yes", "openssl rand", "passgo-password", "netcat-openbsd"} {
		if !strings.Contains(content, required) {
			t.Fatalf("cloud-init is missing %q", required)
		}
	}
	for _, forbidden := range []string{"0.0.0.0:6080", "websockify --web", `printf "passgo`} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("cloud-init contains insecure VNC configuration %q", forbidden)
		}
	}
}
