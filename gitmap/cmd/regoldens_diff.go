package cmd

// Diff summary emitter for `gitmap regoldens`. Reads the git working
// tree to report which `testdata/` golden files were touched by the
// pass-1 regenerate step. Output is intentionally concise: status
// letter, path, and (+adds / -dels) line counts per file plus an
// aggregate totals line. This file is split out so regoldens.go
// stays well under the 200-line cap.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/alimtvnetwork/gitmap-v7/gitmap/constants"
)

// goldenDiffEntry captures one changed testdata/ file. Status uses
// the porcelain letter set: A (added/untracked), M (modified),
// D (deleted), R (renamed), ? (untracked — normalized to A).
type goldenDiffEntry struct {
	status   string
	path     string
	added    int
	deleted  int
}

// goldenDiffPathFragment scopes both `git status` and `git diff`
// output to fixture files. Anything outside `testdata/` is filtered
// because regenerate passes should only touch those paths.
const goldenDiffPathFragment = "testdata/"

// emitGoldenDiffSummary prints the post-pass-1 diff summary. Errors
// from git invocations are surfaced (zero-swallow policy) but never
// fatal — the diff is informational and must not block pass 2.
func emitGoldenDiffSummary() {
	if !isGitWorkingTree() {
		fmt.Fprint(os.Stderr, constants.MsgRegoldensDiffSkipped)
		return
	}
	entries, err := collectGoldenDiffEntries()
	if err != nil {
		fmt.Fprintf(os.Stderr, "regoldens: diff summary failed: %v\n", err)
		return
	}
	fmt.Fprint(os.Stdout, constants.MsgRegoldensDiffHeader)
	if len(entries) == 0 {
		fmt.Fprint(os.Stdout, constants.MsgRegoldensDiffNoChanges)
		return
	}
	printGoldenDiffEntries(entries)
}

// isGitWorkingTree returns true when the current directory is inside
// a git repository. Used to gate the diff feature gracefully.
func isGitWorkingTree() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree") //nolint:gosec // literal argv
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// collectGoldenDiffEntries merges porcelain status (covers
// untracked/added/deleted) with numstat (covers +/- line counts for
// modified files) into a unified per-file record list.
func collectGoldenDiffEntries() ([]goldenDiffEntry, error) {
	statuses, err := readPorcelainStatuses()
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	numstat, err := readNumstatCounts()
	if err != nil {
		return nil, fmt.Errorf("git diff numstat: %w", err)
	}
	return mergeStatusAndNumstat(statuses, numstat), nil
}

// readPorcelainStatuses returns a map of testdata/ path -> status
// letter from `git status --porcelain`. Untracked entries (`??`)
// are normalized to "A" (added) for cleaner display.
func readPorcelainStatuses() (map[string]string, error) {
	out, err := runGitCapture("status", "--porcelain", "--", "*"+goldenDiffPathFragment+"*")
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if !strings.Contains(path, goldenDiffPathFragment) {
			continue
		}
		result[path] = normalizePorcelainStatus(strings.TrimSpace(line[:2]))
	}
	return result, nil
}

// normalizePorcelainStatus collapses git's two-letter porcelain
// codes to a single display letter. Order matters: deletion wins
// over modification when both are present (rare but possible for
// staged-then-deleted files).
func normalizePorcelainStatus(code string) string {
	if code == "??" {
		return "A"
	}
	if strings.Contains(code, "D") {
		return "D"
	}
	if strings.Contains(code, "A") {
		return "A"
	}
	if strings.Contains(code, "R") {
		return "R"
	}
	return "M"
}

// readNumstatCounts returns added/deleted line counts for tracked
// modifications. Untracked files do not appear here — that's why
// readPorcelainStatuses runs in parallel.
func readNumstatCounts() (map[string][2]int, error) {
	out, err := runGitCapture("diff", "--numstat", "--", "*"+goldenDiffPathFragment+"*")
	if err != nil {
		return nil, err
	}
	result := make(map[string][2]int)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		added, _ := strconv.Atoi(fields[0])   // "-" (binary) becomes 0
		deleted, _ := strconv.Atoi(fields[1]) //nolint:errcheck // intentional 0 fallback
		result[fields[2]] = [2]int{added, deleted}
	}
	return result, nil
}

// mergeStatusAndNumstat joins the two maps by path, defaulting line
// counts to 0 for added/deleted/untracked files (numstat omits them).
func mergeStatusAndNumstat(statuses map[string]string, counts map[string][2]int) []goldenDiffEntry {
	entries := make([]goldenDiffEntry, 0, len(statuses))
	for path, status := range statuses {
		c := counts[path]
		entries = append(entries, goldenDiffEntry{
			status: status, path: path, added: c[0], deleted: c[1],
		})
	}
	sortGoldenDiffEntries(entries)
	return entries
}

// runGitCapture executes a git subcommand and returns trimmed stdout.
// stderr is captured into the returned error so failures are visible.
func runGitCapture(args ...string) (string, error) {
	cmd := exec.Command("git", args...) //nolint:gosec // literal "git" + caller-controlled flags
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	return string(out), nil
}
