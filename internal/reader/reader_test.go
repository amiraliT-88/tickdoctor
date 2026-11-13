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
