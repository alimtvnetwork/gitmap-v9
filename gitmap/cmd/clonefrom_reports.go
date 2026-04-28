package cmd

// Report-writing helpers for `gitmap clone-from --execute`. Split
// from clonefrom.go to keep that file under the project's 200-line
// cap. Always writes CSV (unless --no-report); also writes JSON when
// --output terminal is set so the terminal summary can surface both
// paths.

import (
	"fmt"
	"os"

	"github.com/alimtvnetwork/gitmap-v8/gitmap/clonefrom"
	"github.com/alimtvnetwork/gitmap-v8/gitmap/constants"
)

// writeCloneFromReports persists the CSV report (always, unless
// --no-report) and additionally the JSON report when --output
// terminal is set. Returns ("", "") when --no-report skips both.
// Failures are logged to stderr but never abort — clones already
// happened, the reports are bonus.
func writeCloneFromReports(results []clonefrom.Result, cfg cloneFromFlags) (string, string) {
	if cfg.noReport {
		return "", ""
	}
	csvPath := ""
	if p, err := clonefrom.WriteReport(results); err == nil {
		csvPath = p
	} else {
		fmt.Fprintln(os.Stderr, err)
	}

	if cfg.output != constants.OutputTerminal {
		return csvPath, ""
	}

	jsonPath := ""
	if p, err := clonefrom.WriteReportJSON(results); err == nil {
		jsonPath = p
	} else {
		fmt.Fprintln(os.Stderr, err)
	}

	return csvPath, jsonPath
}
