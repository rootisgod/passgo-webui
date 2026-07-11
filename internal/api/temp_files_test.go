package api

import (
	"os"
	"testing"
)

func TestWriteTempCloudInitCreatesPrivateUniqueFiles(t *testing.T) {
	first, err := writeTempCloudInit([]byte("#cloud-config\nfirst: true\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(first)
	second, err := writeTempCloudInit([]byte("#cloud-config\nsecond: true\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(second)

	if first == second {
		t.Fatal("temporary paths must be unique")
	}
	info, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
}
