package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, targets, err := parseFlags()
	if err != nil {
		// parseFlags returns errVersionPrinted as a sentinel for --version.
		if errors.Is(err, errVersionPrinted) {
			return 0
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	exitCode := 0
	for _, target := range targets {
		if err := handleTarget(ctx, target, cfg); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				break
			}
			slog.Error("failed to process target", "target", target, "err", err)
			fmt.Fprintf(os.Stderr, "dog: %s: %v\n", target, err)
			exitCode = 1
		}
	}
	return exitCode
}
