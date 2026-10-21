package main

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it as a string. Thread-unsafe — use only in tests.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out)
}

// ─── streamWithBark ───────────────────────────────────────────────────────────

func TestStreamWithBark_PlainText(t *testing.T) {
	input := "alpha\nbeta\ngamma\n"
	out := captureStdout(t, func() {
		if err := streamWithBark(context.Background(), strings.NewReader(input), ""); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	for _, line := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(out, line) {
			t.Errorf("expected %q in output, got: %q", line, out)
		}
	}
}

func TestStreamWithBark_NoBarkPatternNoAlert(t *testing.T) {
	input := "ERROR: something failed\n"
	// No pattern — no bark, just output.
	out := captureStdout(t, func() {
		if err := streamWithBark(context.Background(), strings.NewReader(input), ""); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "ERROR") {
		t.Errorf("expected line in output, got: %q", out)
	}
}

func TestStreamWithBark_MatchingPattern(t *testing.T) {
	input := "info: ok\nERROR: boom\ninfo: done\n"
	// Capture stderr to verify the bark alert is written.
	oldErr := os.Stderr
	re, we, _ := os.Pipe()
	os.Stderr = we

	out := captureStdout(t, func() {
		if err := streamWithBark(context.Background(), strings.NewReader(input), "ERROR"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	we.Close()
	os.Stderr = oldErr
	errOut, _ := io.ReadAll(re)

	if !strings.Contains(out, "ERROR: boom") {
		t.Errorf("expected matching line in stdout, got: %q", out)
	}
	if !strings.Contains(string(errOut), "BARK!") {
		t.Errorf("expected BARK alert on stderr, got: %q", errOut)
	}
}

func TestStreamWithBark_NoMatchNoAlert(t *testing.T) {
	input := "info: all good\n"
	oldErr := os.Stderr
	re, we, _ := os.Pipe()
	os.Stderr = we

	captureStdout(t, func() {
		if err := streamWithBark(context.Background(), strings.NewReader(input), "ERROR"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	we.Close()
	os.Stderr = oldErr
	errOut, _ := io.ReadAll(re)

	if strings.Contains(string(errOut), "BARK!") {
		t.Errorf("unexpected BARK alert for non-matching line, stderr: %q", errOut)
	}
}

func TestStreamWithBark_InvalidRegex(t *testing.T) {
	err := streamWithBark(context.Background(), strings.NewReader("x"), "[invalid")
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestStreamWithBark_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Even a single-line reader should respect the cancelled context.
	err := streamWithBark(ctx, strings.NewReader("line\n"), "")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestStreamWithBark_EmptyInput(t *testing.T) {
	out := captureStdout(t, func() {
		if err := streamWithBark(context.Background(), strings.NewReader(""), ""); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	if out != "" {
		t.Errorf("expected empty output for empty input, got: %q", out)
	}
}

func TestStreamWithBark_RegexPattern(t *testing.T) {
	input := "2024-01-01 INFO start\n2024-01-01 WARN slow\n2024-01-01 ERROR fail\n"
	oldErr := os.Stderr
	re, we, _ := os.Pipe()
	os.Stderr = we

	captureStdout(t, func() {
		if err := streamWithBark(context.Background(), strings.NewReader(input), `(WARN|ERROR)`); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	we.Close()
	os.Stderr = oldErr
	errOut, _ := io.ReadAll(re)

	// Two matching lines → two BARK alerts.
	count := strings.Count(string(errOut), "BARK!")
	if count != 2 {
		t.Errorf("expected 2 BARK alerts, got %d; stderr: %q", count, errOut)
	}
}

func TestStreamWithBark_LongLine(t *testing.T) {
	// Line within maxScanToken (1 MiB) should not error.
	long := strings.Repeat("a", 512*1024)
	out := captureStdout(t, func() {
		if err := streamWithBark(context.Background(), strings.NewReader(long+"\n"), ""); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	if len(out) < 512*1024 {
		t.Errorf("expected full long line in output, got %d bytes", len(out))
	}
}
