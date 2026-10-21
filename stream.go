package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
)

// streamWithBark streams r line-by-line to stdout. When barkPattern is set,
// any matching line triggers a terminal bell + stderr alert.
func streamWithBark(ctx context.Context, r io.Reader, barkPattern string) error {
	var barkRe *regexp.Regexp
	if barkPattern != "" {
		var err error
		barkRe, err = regexp.Compile(barkPattern)
		if err != nil {
			return fmt.Errorf("invalid bark pattern %q: %w", barkPattern, err)
		}
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, maxScanToken), maxScanToken)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if barkRe != nil && barkRe.MatchString(line) {
			fmt.Fprintf(os.Stderr, "\a🐶 \033[1;31m[BARK! matched /%s/]\033[0m\n", barkPattern)
		}
		fmt.Println(line)
	}
	return scanner.Err()
}
