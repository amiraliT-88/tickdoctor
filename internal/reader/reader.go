package reader

import (
	"bufio"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type RotatedFile struct {
	Path      string
	FirstTime time.Time
	ModTime   time.Time
}

func OpenStream(path string) (io.ReadCloser, *bufio.Scanner, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}

	var r io.ReadCloser = f
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, nil, err
		}
		r = &gzipReadCloser{Reader: gz, closer: f}
	}

	scanner := bufio.NewScanner(r)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	return r, scanner, nil
}

type gzipReadCloser struct {
	*gzip.Reader
	closer io.Closer
}

func (g *gzipReadCloser) Close() error {
	_ = g.Reader.Close()
	return g.closer.Close()
}

func DiscoverRotatedLogs(basePath string) ([]string, error) {
	fi, err := os.Stat(basePath)
	if err != nil {
		return nil, err
	}

	dir := basePath
	prefix := "gc"
	if !fi.IsDir() {
		dir = filepath.Dir(basePath)
		prefix = strings.TrimSuffix(filepath.Base(basePath), filepath.Ext(basePath))
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var matched []RotatedFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) && !strings.Contains(name, "gc") {
			continue
		}

		fullPath := filepath.Join(dir, name)
		info, err := e.Info()
		if err != nil {
			continue
		}

		rf := RotatedFile{
			Path:    fullPath,
			ModTime: info.ModTime(),
		}
		matched = append(matched, rf)
	}

	if len(matched) == 0 {
		return []string{basePath}, nil
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].ModTime.Before(matched[j].ModTime)
	})

	res := make([]string, len(matched))
	for i, m := range matched {
		res[i] = m.Path
	}
	return res, nil
}
