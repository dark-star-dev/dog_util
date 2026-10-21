package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

// handleTarget resolves a target string to a reader and dispatches to the
// appropriate processor.
func handleTarget(ctx context.Context, target string, cfg Config) error {
	var reader io.ReadCloser
	var isLocalFile bool

	switch {
	case target == "-":
		reader = io.NopCloser(os.Stdin)
		slog.Debug("reading from stdin")

	case strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://"):
		r, err := fetchHTTP(ctx, target, "")
		if err != nil {
			return err
		}
		reader = r

	case strings.HasPrefix(target, "gh:"):
		rawURL, err := parseGitHubTarget(target)
		if err != nil {
			return err
		}
		slog.Debug("resolved GitHub URL", "url", rawURL)
		r, err := fetchHTTP(ctx, rawURL, os.Getenv("GITHUB_TOKEN"))
		if err != nil {
			return err
		}
		reader = r

	default:
		f, err := os.Open(target)
		if err != nil {
			return fmt.Errorf("open: %w", err)
		}
		reader = f
		isLocalFile = true
		slog.Debug("opened local file", "path", target)
	}
	defer reader.Close()

	if cfg.Follow && isLocalFile {
		return followFile(ctx, target, cfg)
	}

	return processStream(ctx, reader, cfg)
}

// fetchHTTP performs a GET with the shared hardened client. An optional bearer
// token is added when non-empty.
func fetchHTTP(ctx context.Context, url, token string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	slog.Debug("HTTP GET", "url", url)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
	}
	slog.Debug("HTTP response", "status", resp.StatusCode, "content-type", resp.Header.Get("Content-Type"))
	return resp.Body, nil
}

// parseGitHubTarget converts a gh:owner/repo[@ref]/path shorthand into a
// raw.githubusercontent.com URL.
func parseGitHubTarget(target string) (string, error) {
	trimmed := strings.TrimPrefix(target, "gh:")
	if trimmed == "" {
		return "", fmt.Errorf("empty github target after 'gh:'")
	}

	atIdx := strings.Index(trimmed, "@")

	var ownerRepo, ref, filePath string
	if atIdx != -1 {
		ownerRepo = trimmed[:atIdx]
		rest := trimmed[atIdx+1:]
		slashIdx := strings.Index(rest, "/")
		if slashIdx == -1 {
			return "", fmt.Errorf("missing file path in github target after ref")
		}
		ref = rest[:slashIdx]
		filePath = rest[slashIdx+1:]
	} else {
		parts := strings.SplitN(trimmed, "/", 3)
		if len(parts) < 3 {
			return "", fmt.Errorf("invalid github format — expected gh:owner/repo/path or gh:owner/repo@ref/path")
		}
		ownerRepo = parts[0] + "/" + parts[1]
		ref = "main"
		filePath = parts[2]
	}

	if ownerRepo == "" || ref == "" || filePath == "" {
		return "", fmt.Errorf("incomplete github target: owner/repo=%q ref=%q path=%q", ownerRepo, ref, filePath)
	}

	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", ownerRepo, ref, filePath), nil
}
