package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// ─── isText ───────────────────────────────────────────────────────────────────

func TestIsText_Empty(t *testing.T) {
	if !isText(nil) {
		t.Error("nil slice should be text")
	}
	if !isText([]byte{}) {
		t.Error("empty slice should be text")
	}
}

func TestIsText_PlainUTF8(t *testing.T) {
	if !isText([]byte("hello world\n")) {
		t.Error("plain ASCII should be text")
	}
}

func TestIsText_ValidUTF8NonASCII(t *testing.T) {
	if !isText([]byte("héllo wörld")) {
		t.Error("valid UTF-8 non-ASCII should be text")
	}
}

func TestIsText_NullByte(t *testing.T) {
	if isText([]byte("hel\x00lo")) {
		t.Error("data with null byte should not be text")
	}
}

func TestIsText_InvalidUTF8(t *testing.T) {
	if isText([]byte{0xff, 0xfe, 0x00}) {
		t.Error("invalid UTF-8 with null should not be text")
	}
}

func TestIsText_InvalidUTF8NoNull(t *testing.T) {
	// 0xff is not valid UTF-8 but has no null byte.
	if isText([]byte{0xff, 0xfe}) {
		t.Error("invalid UTF-8 without null should not be text")
	}
}

// ─── isJWT ────────────────────────────────────────────────────────────────────

func TestIsJWT_Valid(t *testing.T) {
	// Minimal structurally-valid JWT-like string.
	tok := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	if !isJWT(tok) {
		t.Errorf("expected isJWT=true for %q", tok)
	}
}

func TestIsJWT_OnlyTwoParts(t *testing.T) {
	if isJWT("eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0") {
		t.Error("two-part string should not be JWT")
	}
}

func TestIsJWT_WrongPrefix(t *testing.T) {
	if isJWT("abc.def.ghi") {
		t.Error("string without eyJ prefix should not be JWT")
	}
}

func TestIsJWT_ContainsSpace(t *testing.T) {
	if isJWT("eyJhbGci OiJ.eyJzdWIiOi.sig") {
		t.Error("JWT with space should not match")
	}
}

func TestIsJWT_ContainsNewline(t *testing.T) {
	if isJWT("eyJhbGciOiJ\neyJzdWIiOi.sig") {
		t.Error("JWT with newline should not match")
	}
}

func TestIsJWT_FourParts(t *testing.T) {
	if isJWT("eyJa.eyJb.sig.extra") {
		t.Error("four-part string should not be JWT")
	}
}

// ─── processStream ────────────────────────────────────────────────────────────

func TestProcessStream_PlainText(t *testing.T) {
	input := "hello\nworld\n"
	out := captureStdout(t, func() {
		cfg := Config{Sniff: true}
		if err := processStream(context.Background(), strings.NewReader(input), cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "hello") || !strings.Contains(out, "world") {
		t.Errorf("expected plain text passthrough, got: %q", out)
	}
}

func TestProcessStream_RawBypassesSniff(t *testing.T) {
	// Put something that looks like JSON but pass Raw=true — should still emit
	// the raw bytes without formatting.
	input := `{"key":"value"}`
	out := captureStdout(t, func() {
		cfg := Config{Raw: true}
		if err := processStream(context.Background(), strings.NewReader(input), cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	// Raw mode — we should see the literal JSON on one line.
	if !strings.Contains(out, `{"key":"value"}`) {
		t.Errorf("expected raw output, got: %q", out)
	}
	// It must NOT be indented.
	if strings.Contains(out, "\n  ") {
		t.Errorf("raw mode should not indent JSON, got: %q", out)
	}
}

func TestProcessStream_JSON(t *testing.T) {
	input := `{"name":"dog","version":2}`
	out := captureStdout(t, func() {
		cfg := Config{Sniff: true}
		if err := processStream(context.Background(), strings.NewReader(input), cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"name"`) || !strings.Contains(out, `"dog"`) {
		t.Errorf("expected indented JSON output, got: %q", out)
	}
	// Should be indented.
	if !strings.Contains(out, "  ") {
		t.Errorf("expected indentation in JSON output, got: %q", out)
	}
}

func TestProcessStream_JSONArray(t *testing.T) {
	input := `[1,2,3]`
	out := captureStdout(t, func() {
		cfg := Config{Sniff: true}
		if err := processStream(context.Background(), strings.NewReader(input), cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "1") {
		t.Errorf("expected JSON array output, got: %q", out)
	}
}

func TestProcessStream_InvalidJSONFallsToText(t *testing.T) {
	// Starts with '{' but is not valid JSON — should fall through to plain text.
	input := `{not valid json`
	out := captureStdout(t, func() {
		cfg := Config{Sniff: true}
		if err := processStream(context.Background(), strings.NewReader(input), cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "{not valid json") {
		t.Errorf("invalid JSON should stream as text, got: %q", out)
	}
}

func TestProcessStream_Binary(t *testing.T) {
	// Null byte signals binary.
	input := []byte("PNG\x00data\xff")
	err := processStream(context.Background(), bytes.NewReader(input), Config{Sniff: true})
	// Binary processing writes to stderr only and returns nil.
	if err != nil {
		t.Errorf("unexpected error for binary data: %v", err)
	}
}

func TestProcessStream_JWT(t *testing.T) {
	// eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9 = {"alg":"HS256","typ":"JWT"}
	// eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ
	tok := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9" +
		".eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ" +
		".SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

	out := captureStdout(t, func() {
		cfg := Config{Sniff: true}
		if err := processStream(context.Background(), strings.NewReader(tok), cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "HEADER") {
		t.Errorf("expected JWT HEADER in output, got: %q", out)
	}
	if !strings.Contains(out, "PAYLOAD") {
		t.Errorf("expected JWT PAYLOAD in output, got: %q", out)
	}
}

func TestProcessStream_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := processStream(ctx, strings.NewReader("line\n"), Config{Raw: true})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// ─── formatJWT ────────────────────────────────────────────────────────────────

func TestFormatJWT_ValidToken(t *testing.T) {
	tok := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9" +
		".eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ" +
		".SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

	out := captureStdout(t, func() {
		if err := formatJWT(tok); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "HS256") {
		t.Errorf("expected alg field in output, got: %q", out)
	}
	if !strings.Contains(out, "John Doe") {
		t.Errorf("expected name claim in output, got: %q", out)
	}
	if !strings.Contains(out, "SIGNATURE") {
		t.Errorf("expected SIGNATURE section, got: %q", out)
	}
}

func TestFormatJWT_MalformedOnePart(t *testing.T) {
	err := formatJWT("notajwt")
	if err == nil {
		t.Error("expected error for malformed JWT with only one part")
	}
}

func TestFormatJWT_BadBase64Header(t *testing.T) {
	// "!!!" is not valid base64url.
	err := formatJWT("!!!.eyJzdWIiOiJ4In0.sig")
	if err == nil {
		t.Error("expected error for bad base64 header segment")
	}
}

func TestFormatJWT_SignaturePreviewTruncated(t *testing.T) {
	tok := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9" +
		".eyJzdWIiOiIxMjM0NTY3ODkwIn0" +
		".averylongsignaturevaluethatexceedssixteencharacters"

	out := captureStdout(t, func() {
		if err := formatJWT(tok); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	// Signature preview must be truncated with "…"
	if !strings.Contains(out, "…") {
		t.Errorf("expected truncation ellipsis in signature preview, got: %q", out)
	}
}
