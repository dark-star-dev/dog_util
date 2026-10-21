package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFollowFile_EmitsNewLines verifies that followFile picks up lines appended
// after it starts. The test writes initial content, starts the follower in a
// goroutine, appends more lines, waits briefly, then cancels the context.
func TestFollowFile_EmitsNewLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// Write some initial content that will be seeked past.
	fmt.Fprintln(f, "initial line — should not appear")
	f.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Capture stdout.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan error, 1)
	go func() {
		cfg := Config{PollMs: 20}
		done <- followFile(ctx, path, cfg)
	}()

	// Give the follower time to open and seek to EOF.
	time.Sleep(50 * time.Millisecond)

	// Append new lines.
	af, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(af, "new line one")
	fmt.Fprintln(af, "new line two")
	af.Close()

	// Give the follower time to pick them up.
	time.Sleep(100 * time.Millisecond)
	cancel()

	if err := <-done; err != nil && err != context.Canceled {
		t.Fatalf("followFile returned unexpected error: %v", err)
	}

	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	if !strings.Contains(out, "new line one") {
		t.Errorf("expected 'new line one' in output, got: %q", out)
	}
	if !strings.Contains(out, "new line two") {
		t.Errorf("expected 'new line two' in output, got: %q", out)
	}
	if strings.Contains(out, "initial line") {
		t.Errorf("initial content should not appear (follow seeks to EOF first), got: %q", out)
	}
}

// TestFollowFile_MissingFile verifies an error is returned immediately.
func TestFollowFile_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-file.log")
	err := followFile(context.Background(), path, Config{PollMs: 20})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// TestFollowFile_ContextCancelledBeforeAppend verifies that cancelling the
// context stops the follower cleanly.
func TestFollowFile_ContextCancelledBeforeAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cancel.log")
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- followFile(ctx, path, Config{PollMs: 20})
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			t.Fatalf("unexpected error after cancel: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("followFile did not stop within 500ms after context cancel")
	}
}

// TestFollowFile_InvalidBarkPattern verifies that an invalid regex is rejected
// immediately without blocking.
func TestFollowFile_InvalidBarkPattern(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := Config{PollMs: 20, Bark: "[invalid"}
	err := followFile(context.Background(), path, cfg)
	if err == nil {
		t.Fatal("expected error for invalid bark regex")
	}
}

// TestFollowFile_TruncationReopens verifies that when the file is truncated
// (simulating log rotation), the follower re-opens it and picks up new content.
func TestFollowFile_TruncationReopens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rotating.log")

	// Create the file with some initial data.
	if err := os.WriteFile(path, []byte("old content\n"), 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	old := os.Stdout
	pr, pw, _ := os.Pipe()
	os.Stdout = pw

	done := make(chan error, 1)
	go func() {
		done <- followFile(ctx, path, Config{PollMs: 20})
	}()

	// Let it seek to EOF.
	time.Sleep(50 * time.Millisecond)

	// Truncate — simulate rotation by replacing with a shorter file.
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	// Wait for the truncation to be detected and the file to be reopened.
	time.Sleep(80 * time.Millisecond)

	// Append content to the newly "rotated" file.
	af, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(af, "post-rotation line")
	af.Close()

	time.Sleep(80 * time.Millisecond)
	cancel()

	if err := <-done; err != nil && err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}

	pw.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := pr.Read(buf)
	out := string(buf[:n])

	if !strings.Contains(out, "post-rotation line") {
		t.Errorf("expected post-rotation content, got: %q", out)
	}
}
