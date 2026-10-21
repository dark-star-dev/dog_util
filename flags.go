package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
)

// errVersionPrinted is returned by parseFlags when --version is requested,
// signalling the caller to exit 0 without printing an error.
var errVersionPrinted = errors.New("version printed")

// parseFlags parses CLI flags, configures the global slog default, and returns
// the resolved Config and the list of targets to process.
func parseFlags() (Config, []string, error) {
	fs := flag.NewFlagSet("dog", flag.ContinueOnError)

	follow  := fs.Bool("f", false, "Follow file changes (like tail -f)")
	followL := fs.Bool("follow", false, "Follow file changes (like tail -f)")
	sniff   := fs.Bool("s", true, "Auto-detect and format content (JSON, JWT, binary)")
	sniffL  := fs.Bool("sniff", true, "Auto-detect and format content")
	raw     := fs.Bool("raw", false, "Output raw bytes without sniffing or formatting")
	bark    := fs.String("b", "", "Bark (alert) on regex pattern match")
	barkL   := fs.String("bark", "", "Bark (alert) on regex pattern match")
	pollMs  := fs.Int("poll-ms", defaultPollMs, "Follow mode poll interval in milliseconds")
	verbose := fs.Bool("v", false, "Enable verbose (debug) logging")
	ver     := fs.Bool("version", false, "Print version and exit")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "dog %s — smarter cat for files, URLs, and GitHub\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage: dog [OPTIONS] <TARGET>...\n\n")
		fmt.Fprintf(os.Stderr, "TARGETS:\n")
		fmt.Fprintf(os.Stderr, "  <file>                   Local file path or - for stdin\n")
		fmt.Fprintf(os.Stderr, "  http[s]://...            HTTP/HTTPS URL\n")
		fmt.Fprintf(os.Stderr, "  gh:owner/repo[@ref]/path GitHub raw file\n\n")
		fmt.Fprintf(os.Stderr, "OPTIONS:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment:\n")
		fmt.Fprintf(os.Stderr, "  GITHUB_TOKEN  Bearer token for authenticated GitHub requests\n")
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return Config{}, nil, err
	}

	if *ver {
		fmt.Println("dog", version)
		return Config{}, nil, errVersionPrinted
	}

	logLevel := slog.LevelWarn
	if *verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	barkPattern := *bark
	if barkPattern == "" {
		barkPattern = *barkL
	}

	cfg := Config{
		Follow:   *follow || *followL,
		Sniff:    (*sniff || *sniffL) && !*raw,
		Raw:      *raw,
		Bark:     barkPattern,
		PollMs:   *pollMs,
		LogLevel: logLevel,
	}

	targets := fs.Args()
	if len(targets) == 0 {
		targets = []string{"-"}
	}

	return cfg, targets, nil
}
