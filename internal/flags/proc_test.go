package flags

import (
	"os"
	"strings"
	"testing"
)

func TestReadProcessCommandLineSelf(t *testing.T) {
	pid := os.Getpid()
	cmd, err := ReadProcessCommandLine(pid)
	if err != nil {
		t.Skipf("skipping ReadProcessCommandLine test (inaccessible or restricted environment): %v", err)
	}

	if cmd == "" {
		t.Errorf("expected non-empty command line for current process")
	}

	if !strings.Contains(strings.ToLower(cmd), "test") && !strings.Contains(strings.ToLower(cmd), "flags") {
		t.Logf("retrieved cmdline: %s", cmd)
	}
}

func TestReadProcessCommandLineInvalid(t *testing.T) {
	_, err := ReadProcessCommandLine(-1)
	if err == nil {
		t.Errorf("expected error for negative pid")
	}
}
