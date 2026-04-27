package clonefrom

// summary_terminal.go — enriched summary block emitted when the
// user passes `--output terminal` to `clone-from --execute`. Lives
// in its own file so summary.go stays under the 200-line cap and
// the legacy RenderSummary path keeps a single, focused home.
//
// What this file adds on top of RenderSummary:
//
//   1. A top-line "found" count — the number of repos in the
//      manifest after dedup. The user's manifest is the source of
//      truth for "found"; status counts come AFTER.
//   2. A per-URL-scheme tally (https / http / ssh / git / file /
//      scp / other) so users running mixed-protocol manifests can
//      see at a glance how the rows split. Scheme detection is
//      consistent with validate.go's looksLikeGitURL — same prefix
//      matching + scp-style detector — so a row that parsed as
//      valid is also classified into one bucket.
//   3. Both report paths (CSV and JSON), since terminal mode now
//      writes the JSON envelope alongside the CSV. RenderSummary's
//      single reportPath argument can't carry both.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alimtvnetwork/gitmap-v7/gitmap/constants"
)

// WriteReportJSON persists the result set as a versioned JSON
// envelope under .gitmap/. Mirrors WriteReport's contract: same
// dir, same timestamp suffix (so the CSV and JSON for one run sort
// adjacently), absolute path on success, ("", err) on failure so
// callers can decide whether to surface the failure (they should:
// the JSON path is shown in the terminal summary).
func WriteReportJSON(results []Result) (string, error) {
	dir := filepath.Join(".gitmap")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf(constants.ErrCloneFromReportMkdir, dir, err)
	}
	name := fmt.Sprintf(constants.CloneFromReportJSONNameFmt, time.Now().Unix())
	full := filepath.Join(dir, name)
	f, err := os.Create(full)
	if err != nil {
		return "", fmt.Errorf(constants.ErrCloneFromReportCreate, full, err)
	}
	defer f.Close()
	if err := writeReportRowsJSON(f, results); err != nil {
		return "", err
	}
	abs, _ := filepath.Abs(full)

	return abs, nil
}

// RenderSummaryTerminal writes the enriched terminal-mode summary
// block. csvPath / jsonPath may each be empty (write skipped or
// failed); the renderer substitutes a single "(skipped …)" line
// when BOTH are empty so the report section never disappears
// entirely. Returns the first write error so a closed pipe
// surfaces immediately instead of silently truncating.
func RenderSummaryTerminal(w io.Writer, results []Result,
	csvPath, jsonPath string) error {
	if err := writeTermSummaryHead(w, results); err != nil {
		return err
	}
	if err := writeTermSummarySchemes(w, results); err != nil {
		return err
	}
	if err := writeTermSummaryStatus(w, results); err != nil {
		return err
	}

	return writeTermSummaryReports(w, csvPath, jsonPath)
}

// writeTermSummaryHead emits the banner + the "found N repo(s)"
// line. Split out so RenderSummaryTerminal stays under the per-
// function budget.
func writeTermSummaryHead(w io.Writer, results []Result) error {
	if _, err := io.WriteString(w, constants.CloneFromTermSummaryHeader); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, constants.CloneFromTermSummaryFoundFmt, len(results))

	return err
}

// writeTermSummarySchemes emits the "by mode:" header followed by
// one line per scheme in stable order. Schemes with zero count are
// omitted to keep the block tight on small manifests; the order is
// fixed so a manifest that only has https + ssh always renders in
// the same sequence regardless of input ordering.
func writeTermSummarySchemes(w io.Writer, results []Result) error {
	tally := tallySchemes(results)
	if _, err := io.WriteString(w, constants.CloneFromTermSummarySchemeHeader); err != nil {
		return err
	}
	for _, scheme := range schemeOrder() {
		count := tally[scheme]
		if count == 0 {
			continue
		}
		if _, err := fmt.Fprintf(w,
			constants.CloneFromTermSummarySchemeRowFmt, scheme, count); err != nil {
			return err
		}
	}

	return nil
}

// writeTermSummaryStatus emits the one-line status tally inside the
// terminal summary block. Numbers come from tallyResults (defined in
// summary.go) so this renderer and the legacy RenderSummary can
// never disagree on the counts.
func writeTermSummaryStatus(w io.Writer, results []Result) error {
	ok, skipped, failed := tallyResults(results)
	_, err := fmt.Fprintf(w, constants.CloneFromTermSummaryStatusFmt,
		ok, skipped, failed, len(results))

	return err
}

// writeTermSummaryReports renders zero, one, or two report-path
// lines. When both paths are empty, a single "(skipped — …)" line
// keeps the section visible so the summary shape is predictable.
func writeTermSummaryReports(w io.Writer, csvPath, jsonPath string) error {
	if len(csvPath) == 0 && len(jsonPath) == 0 {
		_, err := io.WriteString(w, constants.CloneFromTermSummaryReportNone)

		return err
	}
	if len(csvPath) > 0 {
		if _, err := fmt.Fprintf(w,
			constants.CloneFromTermSummaryReportFmt, "csv ", csvPath); err != nil {
			return err
		}
	}
	if len(jsonPath) > 0 {
		if _, err := fmt.Fprintf(w,
			constants.CloneFromTermSummaryReportFmt, "json", jsonPath); err != nil {
			return err
		}
	}

	return nil
}

// schemeOrder returns the canonical render order for the per-mode
// tally. https first because it's the most common in practice, ssh
// next (the typical interactive-developer alternative), then the
// less-common modes, with "other" always last as the catch-all.
func schemeOrder() []string {
	return []string{
		constants.CloneFromSchemeHTTPS,
		constants.CloneFromSchemeHTTP,
		constants.CloneFromSchemeSSH,
		constants.CloneFromSchemeSCP,
		constants.CloneFromSchemeGit,
		constants.CloneFromSchemeFile,
		constants.CloneFromSchemeOther,
	}
}

// tallySchemes counts each result's URL by scheme. Public on the
// package surface (lowercase fine — it's a helper, not API) so the
// terminal renderer + future consumers (e.g. a JSON-mode summary)
// share one classification rule.
func tallySchemes(results []Result) map[string]int {
	out := make(map[string]int, len(schemeOrder()))
	for _, r := range results {
		out[ClassifyScheme(r.Row.URL)]++
	}

	return out
}

// ClassifyScheme picks one bucket for a URL. Logic mirrors
// validate.looksLikeGitURL so a URL that survived parse-time
// validation always lands in a known bucket; truly unrecognised
// strings (validation lets them through only if they have an scp-
// shaped colon) end up as "other". Exported so tests + any future
// per-row preview can reuse the rule.
func ClassifyScheme(url string) string {
	url = strings.TrimSpace(url)
	if hit, ok := matchKnownScheme(url); ok {
		return hit
	}
	if looksLikeSCP(url) {
		return constants.CloneFromSchemeSCP
	}

	return constants.CloneFromSchemeOther
}

// matchKnownScheme walks the prefix table once. Kept tiny so
// ClassifyScheme stays under the function-length budget and the
// table itself is the single source of truth for known schemes.
func matchKnownScheme(url string) (string, bool) {
	prefixes := []struct{ prefix, scheme string }{
		{"https://", constants.CloneFromSchemeHTTPS},
		{"http://", constants.CloneFromSchemeHTTP},
		{"ssh://", constants.CloneFromSchemeSSH},
		{"git://", constants.CloneFromSchemeGit},
		{"file://", constants.CloneFromSchemeFile},
	}
	for _, p := range prefixes {
		if strings.HasPrefix(url, p.prefix) {
			return p.scheme, true
		}
	}

	return "", false
}
