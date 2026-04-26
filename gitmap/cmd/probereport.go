package cmd

// probereport.go — output, JSON shaping, and persistence helpers for
// `gitmap probe`. Split out of probe.go to honor the 200-line per-file
// budget. The dispatcher (runProbe) and the worker pool live in
// probe.go; everything here is single-repo concerns that the workers
// call into. tallyProbe is the only function that mutates shared
// state — its caller (probeWorker in probe.go) holds a mutex around
// each call, so this file does not import sync.

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/alimtvnetwork/gitmap-v7/gitmap/constants"
	"github.com/alimtvnetwork/gitmap-v7/gitmap/model"
	"github.com/alimtvnetwork/gitmap-v7/gitmap/probe"
	"github.com/alimtvnetwork/gitmap-v7/gitmap/store"
)

// probeJSONEntry is a single repo-level result emitted under `--json`.
// Embeds the result + repo identity so a CI consumer can join on either.
type probeJSONEntry struct {
	RepoID         int64  `json:"repoId"`
	Slug           string `json:"slug"`
	AbsolutePath   string `json:"absolutePath"`
	NextVersionTag string `json:"nextVersionTag"`
	NextVersionNum int64  `json:"nextVersionNum"`
	Method         string `json:"method"`
	IsAvailable    bool   `json:"isAvailable"`
	Error          string `json:"error,omitempty"`
}

// executeOneProbe runs a single probe and persists it, mirroring the
// missing-URL handling that the sequential loop used.
func executeOneProbe(db *store.DB, repo model.ScanRecord) probe.Result {
	url := pickProbeURL(repo)
	if url == "" {
		result := probe.Result{Method: constants.ProbeMethodNone, Error: fmt.Sprintf(constants.ErrProbeMissingURL, repo.Slug)}
		recordProbeResult(db, repo, result)

		return result
	}
	result := probe.RunOne(url)
	recordProbeResult(db, repo, result)

	return result
}

// makeProbeEntry converts a probe.Result + repo into a JSON-friendly row.
func makeProbeEntry(repo model.ScanRecord, r probe.Result) probeJSONEntry {
	return probeJSONEntry{
		RepoID:         repo.ID,
		Slug:           repo.Slug,
		AbsolutePath:   repo.AbsolutePath,
		NextVersionTag: r.NextVersionTag,
		NextVersionNum: r.NextVersionNum,
		Method:         r.Method,
		IsAvailable:    r.IsAvailable,
		Error:          r.Error,
	}
}

// emitProbeJSON dumps the collected entries as indented JSON to stdout.
func emitProbeJSON(entries []probeJSONEntry) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

// pickProbeURL prefers HTTPS (less auth friction in CI), falls back to SSH.
func pickProbeURL(r model.ScanRecord) string {
	if r.HTTPSUrl != "" {
		return r.HTTPSUrl
	}

	return r.SSHUrl
}

// recordProbeResult persists the probe row, logging-but-not-exiting on error.
func recordProbeResult(db *store.DB, repo model.ScanRecord, result probe.Result) {
	if err := db.RecordVersionProbe(result.AsModel(repo.ID)); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
}

// tallyProbe updates the running counters and (unless jsonOut) prints the
// per-repo summary line. Caller is responsible for serialising access to
// the counters; with the worker pool that's `counterMu` in runProbePool.
func tallyProbe(repo model.ScanRecord, r probe.Result, ok, none, fail int, jsonOut bool) (int, int, int) {
	if r.Error != "" {
		if !jsonOut {
			fmt.Printf(constants.MsgProbeFailFmt, repo.Slug, r.Error)
		}
		return ok, none, fail + 1
	}
	if r.IsAvailable {
		if !jsonOut {
			fmt.Printf(constants.MsgProbeOkFmt, repo.Slug, r.NextVersionTag, r.Method)
		}
		return ok + 1, none, fail
	}
	if !jsonOut {
		fmt.Printf(constants.MsgProbeNoneFmt, repo.Slug, r.Method)
	}

	return ok, none + 1, fail
}
