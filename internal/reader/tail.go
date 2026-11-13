package reader

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"
	"time"
)

type TailOptions struct {
	PollInterval  time.Duration
	FromBeginning bool
}

// Tail continuously follows a growing file like 'tail -f'.
func Tail(ctx context.Context, path string, opts TailOptions) (<-chan string, <-chan error) {
	lines := make(chan string, 100)
	errs := make(chan error, 1)

	poll := opts.PollInterval
	if poll <= 0 {
		poll = 150 * time.Millisecond
	}

	go func() {
		defer close(lines)
		defer close(errs)

		f, err := os.Open(path)
		if err != nil {
			errs <- err
			return
		}
		defer f.Close()

		if !opts.FromBeginning {
			if _, err := f.Seek(0, io.SeekEnd); err != nil {
				errs <- err
				return
			}
		}

		r := bufio.NewReader(f)
		var partial strings.Builder

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line, err := r.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					if line != "" {
						partial.WriteString(line)
					}

					fi, statErr := f.Stat()
					if statErr == nil {
						curPos, _ := f.Seek(0, io.SeekCurrent)
						if fi.Size() < curPos {
							f.Seek(0, io.SeekStart)
							r.Reset(f)
							partial.Reset()
						}
					}

					select {
					case <-ctx.Done():
						return
					case <-time.After(poll):
					}
					continue
				}

				errs <- err
				return
			}

			if partial.Len() > 0 {
				partial.WriteString(line)
				line = partial.String()
				partial.Reset()
			}

			trimmed := strings.TrimRight(line, "\r\n")
			select {
			case <-ctx.Done():
				return
			case lines <- trimmed:
			}
		}
	}()

	return lines, errs
}
