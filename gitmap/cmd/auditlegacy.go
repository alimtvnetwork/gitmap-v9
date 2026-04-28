// Package cmd: audit-legacy scans the workspace for forbidden legacy
// strings (default: gitmap-v5 / gitmap-v6 / gitmap-v7) and exits 1 on
// any hit. Designed as a regression guard for remixes / rename commits.
package cmd

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alimtvnetwork/gitmap-v8/gitmap/constants"
)

// auditLegacyHit is one matched line.
type auditLegacyHit struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Pattern string `json:"pattern"`
	Text    string `json:"text"`
}

// auditLegacyOpts holds parsed CLI flags.
type auditLegacyOpts struct {
	Patterns []*regexp.Regexp
	Raw      []string
	Root     string
	AsJSON   bool
}

// runAuditLegacy is the dispatch entry point.
func runAuditLegacy(args []string) {
	checkHelp(constants.CmdAuditLegacy, args)
	opts, err := parseAuditLegacyArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	hits, fileCount, walkErr := scanAuditLegacy(opts)
	if walkErr != nil {
		fmt.Fprintf(os.Stderr, constants.ErrAuditLegacyWalk, opts.Root, walkErr)
		os.Exit(2)
	}
	emitAuditLegacy(opts, hits, fileCount)
	if len(hits) > 0 {
		os.Exit(1)
	}
}

// parseAuditLegacyArgs parses flags into an options struct.
func parseAuditLegacyArgs(args []string) (auditLegacyOpts, error) {
	fs := flag.NewFlagSet(constants.CmdAuditLegacy, flag.ContinueOnError)
	pats := fs.String(constants.FlagAuditLegacyPatterns,
		constants.DefaultAuditLegacyPatterns, constants.FlagDescAuditLegacyPatterns)
	root := fs.String(constants.FlagAuditLegacyPath, ".", constants.FlagDescAuditLegacyPath)
	asJSON := fs.Bool(constants.FlagAuditLegacyJSON, false, constants.FlagDescAuditLegacyJSON)
	if err := fs.Parse(args); err != nil {
		return auditLegacyOpts{}, err
	}
	compiled, raw, err := compileAuditPatterns(*pats)
	if err != nil {
		return auditLegacyOpts{}, err
	}

	return auditLegacyOpts{Patterns: compiled, Raw: raw, Root: *root, AsJSON: *asJSON}, nil
}

// compileAuditPatterns compiles a comma-separated pattern list.
func compileAuditPatterns(csv string) ([]*regexp.Regexp, []string, error) {
	parts := strings.Split(csv, ",")
	out := make([]*regexp.Regexp, 0, len(parts))
	raw := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, nil, fmt.Errorf(constants.ErrAuditLegacyRegex, p, err)
		}
		out = append(out, re)
		raw = append(raw, p)
	}

	return out, raw, nil
}

// scanAuditLegacy walks the root and returns hits + scanned-file count.
func scanAuditLegacy(opts auditLegacyOpts) ([]auditLegacyHit, int, error) {
	var hits []auditLegacyHit
	var fileCount int
	walkErr := filepath.WalkDir(opts.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			return skipAuditDir(d.Name())
		}
		if !isAuditScannable(path) {
			return nil
		}
		fileCount++
		fileHits := scanAuditFile(path, opts.Patterns)
		hits = append(hits, fileHits...)

		return nil
	})

	return hits, fileCount, walkErr
}

// skipAuditDir returns fs.SkipDir for ignored top-level directories.
func skipAuditDir(name string) error {
	switch name {
	case ".git", "node_modules", "dist", "build", "bin", ".next",
		".gitmap", "vendor", "coverage":
		return fs.SkipDir
	}

	return nil
}

// isAuditScannable filters out binary / non-text files by extension.
func isAuditScannable(path string) bool {
	skip := []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico",
		".pdf", ".zip", ".gz", ".tar", ".exe", ".dll", ".so",
		".dylib", ".bin", ".db", ".sqlite", ".woff", ".woff2", ".ttf"}
	low := strings.ToLower(path)
	for _, ext := range skip {
		if strings.HasSuffix(low, ext) {
			return false
		}
	}

	return true
}

// scanAuditFile reads one file and returns all matches.
func scanAuditFile(path string, pats []*regexp.Regexp) []auditLegacyHit {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var hits []auditLegacyHit
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		for _, re := range pats {
			if re.MatchString(line) {
				hits = append(hits, auditLegacyHit{
					File: path, Line: lineNo, Pattern: re.String(), Text: line,
				})
			}
		}
	}

	return hits
}

// emitAuditLegacy prints results in JSON or human format.
func emitAuditLegacy(opts auditLegacyOpts, hits []auditLegacyHit, fileCount int) {
	if opts.AsJSON {
		emitAuditLegacyJSON(opts, hits, fileCount)

		return
	}
	emitAuditLegacyText(opts, hits, fileCount)
}

// emitAuditLegacyJSON prints a JSON report to stdout.
func emitAuditLegacyJSON(opts auditLegacyOpts, hits []auditLegacyHit, fileCount int) {
	report := map[string]any{
		"root":           opts.Root,
		"patterns":       opts.Raw,
		"filesScanned":   fileCount,
		"matchCount":     len(hits),
		"matches":        hits,
		"filesWithMatch": uniqueAuditFiles(hits),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "audit-legacy: json encode failed: %v\n", err)
	}
}

// emitAuditLegacyText prints a human-readable report.
func emitAuditLegacyText(opts auditLegacyOpts, hits []auditLegacyHit, fileCount int) {
	if len(hits) == 0 {
		fmt.Fprintf(os.Stdout, constants.MsgAuditLegacyClean, opts.Root)

		return
	}
	files := uniqueAuditFiles(hits)
	fmt.Fprintf(os.Stdout, constants.MsgAuditLegacyHeader, len(hits), len(files), opts.Raw)
	for _, h := range hits {
		fmt.Fprintf(os.Stdout, constants.MsgAuditLegacyHit, h.File, h.Line, h.Text)
	}
	_ = fileCount
}

// uniqueAuditFiles returns the deduped file list for the report.
func uniqueAuditFiles(hits []auditLegacyHit) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, h := range hits {
		if _, ok := seen[h.File]; ok {
			continue
		}
		seen[h.File] = struct{}{}
		out = append(out, h.File)
	}

	return out
}
