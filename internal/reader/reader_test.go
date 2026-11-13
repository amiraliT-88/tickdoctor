package reader

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenStreamPlain(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.log")
	content := "line 1\r\nline 2\nline 3\n"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	closer, scanner, err := OpenStream(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line 1" || lines[1] != "line 2" || lines[2] != "line 3" {
		t.Fatalf("unexpected line contents: %v", lines)
	}
}

func TestOpenStreamGzip(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.log.gz")
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write([]byte("gz line 1\ngz line 2\n")); err != nil {
		t.Fatal(err)
	}
	gw.Close()

	if err := os.WriteFile(tmp, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	closer, scanner, err := OpenStream(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	if len(lines) != 2 || lines[0] != "gz line 1" {
		t.Fatalf("unexpected gzip content: %v", lines)
	}
}

func TestDiscoverRotatedLogs(t *testing.T) {
	dir := t.TempDir()

	older := filepath.Join(dir, "gc.log.1")
	newer := filepath.Join(dir, "gc.log")
	unrelated := filepath.Join(dir, "other_gc.log")
	lockfile := filepath.Join(dir, "gc.log.lck")

	if err := os.WriteFile(older, []byte("[2026-11-15T10:00:00.000+0000][info][gc] GC(0) ...\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newer, []byte("[2026-11-15T12:00:00.000+0000][info][gc] GC(10) ...\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unrelated, []byte("unrelated\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockfile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := DiscoverRotatedLogs(newer)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 rotated files (excluding other_gc.log and .lck), got %v", files)
	}
	if files[0] != older || files[1] != newer {
		t.Errorf("expected chronological order [%s, %s], got %v", older, newer, files)
	}
}
