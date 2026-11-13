package reader

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTail(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test_tail.log")

	if err := os.WriteFile(logPath, []byte("initial line 1\n"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lines, errs := Tail(ctx, logPath, TailOptions{
		PollInterval:  20 * time.Millisecond,
		FromBeginning: true,
	})

	select {
	case line := <-lines:
		if line != "initial line 1" {
			t.Errorf("expected 'initial line 1', got '%s'", line)
		}
	case err := <-errs:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for initial line")
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open for append: %v", err)
	}
	f.WriteString("streamed line 2\n")
	f.Close()

	select {
	case line := <-lines:
		if line != "streamed line 2" {
			t.Errorf("expected 'streamed line 2', got '%s'", line)
		}
	case err := <-errs:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for streamed line")
	}

	cancel()

	select {
	case <-lines:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for lines channel to close")
	}
}
