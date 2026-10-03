package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"couchverse/internal/hls"
)

// validateHLS checks an HLS presentation (a URL, or a playlist path on disk) the
// way AVPlayer will judge it and returns the process exit code: 1 when there
// are errors, or warnings under -strict.
func validateHLS(args []string, out io.Writer) int {
	fs := flag.NewFlagSet("validate-hls", flag.ContinueOnError)
	fs.SetOutput(out)
	strict := fs.Bool("strict", false, "fail on warnings too")
	maxSegments := fs.Int("max-segments", 0, "parse only the first n segments of each playlist (0: all)")
	quiet := fs.Bool("q", false, "print only issues")
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 {
		fmt.Fprintln(out, "usage: couchverse validate-hls [-strict] [-max-segments n] [-q] <url or playlist path>")
		return 2
	}
	target := fs.Arg(0)
	if !strings.Contains(target, "://") {
		abs, err := filepath.Abs(target)
		if err != nil {
			fmt.Fprintln(out, err)
			return 2
		}
		target = "file://" + abs
	}
	report, err := hls.Validate(context.Background(), hls.HTTPFetcher{}, target, hls.Options{MaxSegments: *maxSegments})
	if err != nil {
		fmt.Fprintln(out, err)
		return 1
	}
	for _, issue := range report.Issues {
		fmt.Fprintln(out, issue)
	}
	errors := len(report.Errors())
	warnings := len(report.Issues) - errors
	if !*quiet {
		fmt.Fprintf(out, "%s: %d playlists, %d segments, %d errors, %d warnings\n",
			fs.Arg(0), report.Playlists, report.Segments, errors, warnings)
	}
	if errors > 0 || *strict && warnings > 0 {
		return 1
	}
	return 0
}
