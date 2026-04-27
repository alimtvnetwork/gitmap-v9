package cmd

// Sort + render helpers for the regoldens diff summary. Split from
// regoldens_diff.go so each file stays well under the 200-line cap
// and each function under the 15-line cap.

import (
	"fmt"
	"os"
	"sort"

	"github.com/alimtvnetwork/gitmap-v7/gitmap/constants"
)

// sortGoldenDiffEntries orders entries deterministically: by status
// (deletions last, additions first) then by path. Stable output
// helps reviewers diff two regoldens runs.
func sortGoldenDiffEntries(entries []goldenDiffEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].status != entries[j].status {
			return goldenDiffStatusRank(entries[i].status) < goldenDiffStatusRank(entries[j].status)
		}
		return entries[i].path < entries[j].path
	})
}

// goldenDiffStatusRank assigns a sort weight per status letter so
// the summary reads added → modified → renamed → deleted.
func goldenDiffStatusRank(status string) int {
	switch status {
	case "A":
		return 0
	case "M":
		return 1
	case "R":
		return 2
	case "D":
		return 3
	}
	return 4
}

// printGoldenDiffEntries emits one line per file plus an aggregate
// totals line. Counts are summed in a single pass for clarity.
func printGoldenDiffEntries(entries []goldenDiffEntry) {
	totals := goldenDiffTotals{count: len(entries)}
	for _, e := range entries {
		fmt.Fprintf(os.Stdout, constants.MsgRegoldensDiffLine,
			e.status, e.path, e.added, e.deleted)
		totals.accumulate(e)
	}
	fmt.Fprintf(os.Stdout, constants.MsgRegoldensDiffTotals,
		totals.count, totals.added, totals.modified, totals.deleted,
		totals.linesAdded, totals.linesDeleted)
}

// goldenDiffTotals aggregates per-status counts and total +/- lines.
type goldenDiffTotals struct {
	count        int
	added        int
	modified     int
	deleted      int
	linesAdded   int
	linesDeleted int
}

// accumulate folds one entry into the running totals. Status counts
// are mutually exclusive per file; line counts always accrue.
func (t *goldenDiffTotals) accumulate(e goldenDiffEntry) {
	switch e.status {
	case "A":
		t.added++
	case "D":
		t.deleted++
	default:
		t.modified++
	}
	t.linesAdded += e.added
	t.linesDeleted += e.deleted
}
