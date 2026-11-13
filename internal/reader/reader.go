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
	stem := "gc.log"
	isDir := fi.IsDir()
	if !isDir {
		dir = filepath.Dir(basePath)
		stem = rotationStem(filepath.Base(basePath))
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
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".lck") || strings.HasSuffix(lower, ".tmp") || strings.HasSuffix(lower, ".bak") {
			continue
		}

		if isDir {
			if name != "gc.log" && !strings.HasPrefix(name, "gc.log.") && !strings.HasPrefix(name, "gc-") {
				continue
			}
		} else {
			if name != stem && !strings.HasPrefix(name, stem+".") {
				continue
			}
		}

		fullPath := filepath.Join(dir, name)
		info, err := e.Info()
		if err != nil {
			continue
		}

		matched = append(matched, RotatedFile{
			Path:      fullPath,
			FirstTime: extractFirstTimestamp(fullPath),
			ModTime:   info.ModTime(),
		})
	}

	if len(matched) == 0 {
		if isDir {
			return nil, os.ErrNotExist
		}
		return []string{basePath}, nil
	}

	sort.Slice(matched, func(i, j int) bool {
		ti, tj := matched[i].FirstTime, matched[j].FirstTime
		if !ti.IsZero() && !tj.IsZero() && !ti.Equal(tj) {
			return ti.Before(tj)
		}
		if !matched[i].ModTime.Equal(matched[j].ModTime) {
			return matched[i].ModTime.Before(matched[j].ModTime)
		}
		return matched[i].Path < matched[j].Path
	})

	res := make([]string, len(matched))
	for i, m := range matched {
		res[i] = m.Path
	}
	return res, nil
}

func rotationStem(name string) string {
	s := name
	if strings.HasSuffix(strings.ToLower(s), ".gz") {
		s = s[:len(s)-3]
	}
	if strings.HasSuffix(strings.ToLower(s), ".current") {
		s = s[:len(s)-len(".current")]
	}
	if idx := strings.LastIndexByte(s, '.'); idx > 0 {
		suffix := s[idx+1:]
		allDigits := len(suffix) > 0
		for i := 0; i < len(suffix); i++ {
			if suffix[i] < '0' || suffix[i] > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			s = s[:idx]
		}
	}
	return s
}

func extractFirstTimestamp(path string) time.Time {
	closer, scanner, err := OpenStream(path)
	if err != nil {
		return time.Time{}
	}
	defer closer.Close()

	layouts := []string{
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02T15:04:05.000",
		time.RFC3339Nano,
		time.RFC3339,
	}

	for i := 0; i < 25 && scanner.Scan(); i++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			end := strings.IndexByte(line, ']')
			if end > 1 {
				tok := strings.TrimSpace(line[1:end])
				for _, l := range layouts {
					if t, err := time.Parse(l, tok); err == nil {
						return t
					}
				}
			}
		}
		if idx := strings.Index(line, ": "); idx > 10 && idx < 40 {
			tok := strings.TrimSpace(line[:idx])
			for _, l := range layouts {
				if t, err := time.Parse(l, tok); err == nil {
					return t
				}
			}
		}
	}
	return time.Time{}
}
