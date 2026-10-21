package main

import (
	"crypto/tls"
	"log/slog"
	"net/http"
	"time"
)

const (
	version       = "0.2.0"
	userAgent     = "dog/" + version + " (github.com/dog_util)"
	sniffPeekSize = 4096 // bytes read ahead for content sniffing
	maxScanToken  = 1 << 20 // 1 MiB per line before hard wrap
	defaultPollMs = 100     // follow mode default poll interval (ms)
	httpTimeout   = 30 * time.Second
	tlsMinVersion = tls.VersionTLS12
)

// Config holds all resolved runtime configuration.
type Config struct {
	Follow   bool
	Sniff    bool
	Raw      bool
	Bark     string
	PollMs   int
	LogLevel slog.Level
}

// httpClient is the shared, hardened HTTP client used for all outbound requests.
var httpClient = &http.Client{
	Timeout: httpTimeout,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tlsMinVersion,
		},
		MaxIdleConns:       20,
		IdleConnTimeout:    90 * time.Second,
		DisableCompression: false,
		ForceAttemptHTTP2:  true,
	},
}
