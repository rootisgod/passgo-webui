package multipass

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadFixture reads a captured multipass output file from testdata/. These
// fixtures are real captures (see the commands in CLAUDE.md or regenerate
// with `multipass <cmd> --format json > pkg/multipass/testdata/<file>.json`).
// Using real captures catches drift when multipass changes its output format.
func loadFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("load fixture %q: %v", name, err)
	}
	return string(data)
}

// discardLogger returns a logger whose output is discarded — tests rarely
// need to assert on log output, but the Client constructor requires a logger.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeRunner returns a CommandRunner that records every call and replies
// from a lookup table keyed by the joined args. Calls that miss the table
// fail the test — this forces each test to declare exactly what multipass
// invocations it expects, catching silent arg changes.
//
// Returns the runner plus a pointer to the captured calls slice so tests
// can assert on exact argv.
func fakeRunner(t *testing.T, cases map[string]string) (CommandRunner, *[][]string) {
	t.Helper()
	var calls [][]string
	runner := func(args ...string) (string, error) {
		calls = append(calls, args)
		key := strings.Join(args, " ")
		if out, ok := cases[key]; ok {
			return out, nil
		}
		t.Errorf("unexpected multipass call: %v", args)
		return "", fmt.Errorf("no fake response for: %v", args)
	}
	return runner, &calls
}

// errRunner returns a CommandRunner that always returns the given error and
// records calls, for testing failure paths.
func errRunner(err error) (CommandRunner, *[][]string) {
	var calls [][]string
	runner := func(args ...string) (string, error) {
		calls = append(calls, args)
		return "", err
	}
	return runner, &calls
}

// noCallRunner returns a CommandRunner that fails the test if called.
// Use when asserting that validation short-circuits before any exec.
func noCallRunner(t *testing.T) CommandRunner {
	t.Helper()
	return func(args ...string) (string, error) {
		t.Errorf("runner should not have been called, but was with: %v", args)
		return "", fmt.Errorf("unexpected call")
	}
}
