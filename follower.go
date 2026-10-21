package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"
)

// followFile tails a local file like `tail -f`, with truncation/rotation
// detection and context-aware shutdown.
func followFile(ctx context.Context, path string, cfg Config) error {
	var barkRe *regexp.Regexp
	if cfg.Bark != "" {
		var err error
		barkRe, err = regexp.Compile(cfg.Bark)
		if err != nil {
			return fmt.Errorf("invalid bark pattern %q: %w", cfg.Bark, err)
		}
	}

	pollInterval := time.Duration(cfg.PollMs) * time.Millisecond
	if pollInterval <= 0 {
		pollInterval = defaultPollMs * time.Millisecond
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	// Seek to the end so we only emit new content.
	pos, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seek: %w", err)
	}
	slog.Debug("following file", "path", path, "start_offset", pos)

	reader := bufio.NewReaderSize(f, 64*1024)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			trimmed := strings.TrimRight(line, "\r\n")
			if barkRe != nil && barkRe.MatchString(trimmed) {
				fmt.Fprintf(os.Stderr, "\a🐶 \033[1;31m[BARK! matched /%s/]\033[0m\n", cfg.Bark)
			}
			fmt.Println(trimmed)
		}

		if errors.Is(readErr, io.EOF) {
			// Check for file truncation / rotation.
			currentPos, _ := f.Seek(0, io.SeekCurrent)
			fi, statErr := os.Stat(path)
			if statErr == nil && fi.Size() < currentPos {
				slog.Info("file truncated or rotated — reopening", "path", path)
				_ = f.Close()
				f, err = os.Open(path)
				if err != nil {
					return fmt.Errorf("reopen after rotation: %w", err)
				}
				reader = bufio.NewReaderSize(f, 64*1024)
			}

			// Wait before polling again.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(pollInterval):
			}
			continue
		}

		if readErr != nil {
			return fmt.Errorf("read: %w", readErr)
		}
	}
}
