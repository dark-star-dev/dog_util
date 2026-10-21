package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── parseGitHubTarget ────────────────────────────────────────────────────────

func TestParseGitHubTarget_WithRef(t *testing.T) {
	got, err := parseGitHubTarget("gh:spf13/cobra@main/README.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://raw.githubusercontent.com/spf13/cobra/main/README.md"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseGitHubTarget_WithoutRef_DefaultsToMain(t *testing.T) {
	got, err := parseGitHubTarget("gh:spf13/cobra/README.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://raw.githubusercontent.com/spf13/cobra/main/README.md"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseGitHubTarget_NestedPath(t *testing.T) {
	got, err := parseGitHubTarget("gh:owner/repo@v1.2.3/cmd/main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://raw.githubusercontent.com/owner/repo/v1.2.3/cmd/main.go"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseGitHubTarget_Empty(t *testing.T) {
	_, err := parseGitHubTarget("gh:")
	if err == nil {
		t.Error("expected error for empty target")
	}
}

func TestParseGitHubTarget_MissingPath(t *testing.T) {
	_, err := parseGitHubTarget("gh:owner/repo@main")
	if err == nil {
		t.Error("expected error when path is missing after ref")
	}
}

func TestParseGitHubTarget_TooFewSegments(t *testing.T) {
	_, err := parseGitHubTarget("gh:owner/repo")
	if err == nil {
		t.Error("expected error for gh:owner/repo with no path")
	}
}

// ─── fetchHTTP ────────────────────────────────────────────────────────────────

func TestFetchHTTP_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != userAgent {
			t.Errorf("missing User-Agent header, got %q", r.Header.Get("User-Agent"))
		}
		fmt.Fprintln(w, "hello")
	}))
	defer srv.Close()

	rc, err := fetchHTTP(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rc.Close()

	body, _ := io.ReadAll(rc)
	if !strings.Contains(string(body), "hello") {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestFetchHTTP_BearerToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		fmt.Fprintln(w, "ok")
	}))
	defer srv.Close()

	rc, err := fetchHTTP(context.Background(), srv.URL, "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc.Close()
}

func TestFetchHTTP_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchHTTP(context.Background(), srv.URL, "")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404, got: %v", err)
	}
}

func TestFetchHTTP_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "too late")
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the request fires

	_, err := fetchHTTP(ctx, srv.URL, "")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// ─── handleTarget (local file) ────────────────────────────────────────────────

func TestHandleTarget_LocalFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "dog-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(f, "line one")
	fmt.Fprintln(f, "line two")
	f.Close()

	// Redirect stdout so we can capture output.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := Config{Sniff: false, Raw: true}
	if err := handleTarget(context.Background(), f.Name(), cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)

	if !strings.Contains(string(out), "line one") {
		t.Errorf("expected 'line one' in output, got: %q", out)
	}
}

func TestHandleTarget_MissingFile(t *testing.T) {
	err := handleTarget(context.Background(), filepath.Join(t.TempDir(), "nope.txt"), Config{})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestHandleTarget_HTTPTarget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "remote content")
	}))
	defer srv.Close()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := Config{Raw: true}
	if err := handleTarget(context.Background(), srv.URL, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)

	if !strings.Contains(string(out), "remote content") {
		t.Errorf("expected 'remote content', got: %q", out)
	}
}
