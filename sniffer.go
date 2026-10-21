package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"unicode/utf8"
)

// processStream sniffs the content of r and routes it to the appropriate
// formatter, then streams the result via streamWithBark.
func processStream(ctx context.Context, r io.Reader, cfg Config) error {
	if cfg.Raw {
		return streamWithBark(ctx, r, cfg.Bark)
	}

	// Peek at the first sniffPeekSize bytes without consuming the stream.
	peekBuf := make([]byte, sniffPeekSize)
	n, err := io.ReadFull(r, peekBuf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("read peek: %w", err)
	}
	sample := peekBuf[:n]

	// Recombine the peeked bytes with the remaining stream for later readers.
	combined := io.MultiReader(bytes.NewReader(sample), r)

	// 1. Binary / non-UTF-8 detection.
	if !isText(sample) {
		slog.Debug("sniffed binary content")
		fmt.Fprintf(os.Stderr, "🐶 [dog: binary data detected — showing first %d bytes as hex dump]\n", n)
		fmt.Print(hex.Dump(sample))
		if n == sniffPeekSize {
			fmt.Fprintln(os.Stderr, "... (truncated — use --raw to stream all bytes)")
		}
		return nil
	}

	// 2. JWT detection.
	if jwtStr := strings.TrimSpace(string(sample)); isJWT(jwtStr) {
		slog.Debug("sniffed JWT token")
		fmt.Fprintln(os.Stderr, "🐶 [dog: JWT token detected]")
		return formatJWT(jwtStr)
	}

	// 3. JSON detection.
	trimmed := bytes.TrimSpace(sample)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		allData, readErr := io.ReadAll(combined)
		if readErr == nil && json.Valid(allData) {
			var pretty bytes.Buffer
			if indentErr := json.Indent(&pretty, allData, "", "  "); indentErr == nil {
				slog.Debug("sniffed valid JSON")
				return streamWithBark(ctx, &pretty, cfg.Bark)
			}
		}
		// Not fully valid JSON — fall through to plain text streaming.
		combined = bytes.NewReader(allData)
	}

	// 4. Plain text streaming.
	return streamWithBark(ctx, combined, cfg.Bark)
}

// isText returns true when data contains no null bytes and is valid UTF-8.
func isText(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	if bytes.IndexByte(data, 0) != -1 {
		return false
	}
	return utf8.Valid(data)
}

// isJWT returns true when s looks like a compact JWT (three base64url segments,
// first starting with "eyJ", no whitespace).
func isJWT(s string) bool {
	parts := strings.Split(s, ".")
	return len(parts) == 3 &&
		strings.HasPrefix(parts[0], "eyJ") &&
		!strings.ContainsAny(s, " \t\n\r")
}

// formatJWT decodes and pretty-prints the header and payload of a JWT token.
func formatJWT(token string) error {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return fmt.Errorf("malformed JWT: expected 3 dot-separated segments")
	}

	decodePart := func(name, seg string) error {
		data, err := base64.RawURLEncoding.DecodeString(seg)
		if err != nil {
			return fmt.Errorf("JWT %s: base64 decode: %w", name, err)
		}
		var pretty bytes.Buffer
		if json.Indent(&pretty, data, "", "  ") == nil {
			fmt.Printf("// ── %s ──\n%s\n\n", name, pretty.String())
		} else {
			fmt.Printf("// ── %s ──\n%s\n\n", name, string(data))
		}
		return nil
	}

	if err := decodePart("HEADER", parts[0]); err != nil {
		return err
	}
	if err := decodePart("PAYLOAD", parts[1]); err != nil {
		return err
	}
	sigPreview := parts[2]
	if len(sigPreview) > 16 {
		sigPreview = sigPreview[:16] + "…"
	}
	fmt.Printf("// ── SIGNATURE ──\n[%s]\n", sigPreview)
	return nil
}
