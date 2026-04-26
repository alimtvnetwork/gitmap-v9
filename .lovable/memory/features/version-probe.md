---
name: version-probe
description: Hybrid HEAD-then-clone version probe (v3.8.0+). v3.134.0 adds a capped worker pool to `gitmap probe` (default 2, cap 3) via `--workers N`.
type: feature
---
# Version Probe (Phase 2.3, v3.8.0)

## Overview

Two changes ship together in v3.8.0:

1. **`gitmap scan` auto-tags every discovered repo** with the `ScanFolderId` of the just-registered scan root. New helper: `cmd/scan.go::tagReposWithScanFolder` calls `EnsureScanFolder(absDir, "", "")` and then `db.TagReposByScanFolder(folder.ID, paths)`. Failures log to stderr but do NOT fail the scan.
2. **New `gitmap probe [<repo-path>|--all]` command** runs the hybrid HEAD-then-clone version probe and persists results into the `VersionProbe` table.

## Probe strategy

Order matters — fall through to the next strategy only when the previous one fails:

| # | Strategy | Command | When it fails |
|---|---|---|---|
| 1 | `ls-remote` | `git ls-remote --tags --sort=-v:refname <url>` | Server rejects unauthenticated probes, returns zero tags, or git exits non-zero |
| 2 | `shallow-clone` | `git clone --depth 1 --filter=blob:none --no-checkout <url>` into `os.MkdirTemp` then `git tag --sort=-v:refname` | Network/auth failure |

The shallow-clone fallback is **treeless** (`--filter=blob:none`) and **checkout-less** (`--no-checkout`) so we only pay for the refs database — no working tree, no blobs.

## Database

`store/version_probe.go` adds three methods on `*DB`:

- `TagReposByScanFolder(scanFolderID int64, paths []string) error` — bulk `UPDATE Repo SET ScanFolderId = ? WHERE AbsolutePath IN (?,?,?)` via interpolated placeholders. No-op when paths is empty.
- `RecordVersionProbe(model.VersionProbe) error` — inserts a row, mapping `IsAvailable bool` to `INTEGER 0|1`.
- `LatestVersionProbe(repoID int64) (model.VersionProbe, error)` — returns `sql.ErrNoRows` when no probe has run yet (caller handles).

## URL preference

`pickProbeURL` prefers `HTTPSUrl` over `SSHUrl` — HTTPS has less auth friction in CI / first-time-ever clones. SSH only kicks in when HTTPS is empty.

## Semver int

`probe.parseSemverInt` packs `vMAJOR.MINOR.PATCH` into `MAJOR*1e6 + MINOR*1e3 + PATCH` for use in `ORDER BY NextVersionNum DESC` queries. Pre-release suffixes (e.g. `1.2.3-rc1`) collapse to the numeric prefix only — display logic should always use `NextVersionTag`, never `NextVersionNum`.

## CLI surface

```
gitmap probe                   # probe every repo in the database (2 workers)
gitmap probe --all             # explicit form of the above
gitmap probe E:\src\my-repo    # probe a single repo by path
gitmap probe --all --json      # JSON array, input order preserved
gitmap probe --all --workers 3 # raise the worker pool (cap = 3)
```

Per-repo line format:
- `✓ <slug> → v1.2.3 (method=ls-remote)`
- `· <slug> → no new version (method=ls-remote)`
- `✗ <slug> → <error>`

Final summary: `✓ Probe complete: <available> available, <unchanged> unchanged, <failed> failed.`

## Foreground worker pool (v3.134.0)

`gitmap probe` runs through a capped worker pool:

- Default `--workers 2` — sweet spot for residential bandwidth.
- Hard cap of 3 (`constants.ProbeMaxWorkers`); higher values clamp with a
  one-line stderr notice.
- Values < 1 are rejected at parse time.
- JSON output preserves input order regardless of completion order
  (each worker writes to its own pre-allocated slot in the entries
  slice). Human progress lines print as workers complete.
- Counter updates and the per-line print share a single `counterMu` so
  totals stay coherent and stdout lines never interleave mid-print.

The background runner used by `scan` (`probe.BackgroundRunner`) is
unchanged and still defaults to 3 workers — its job pattern (long-lived,
fire-and-forget) tolerates more parallelism than the foreground command.

## Files

- `gitmap/probe/probe.go` — `RunOne`, `tryLsRemote`, `parseFirstTag`, `parseSemverInt`, `Result.AsModel`
- `gitmap/probe/clone.go` — `tryShallowClone`, `summarize`
- `gitmap/store/version_probe.go` — DB methods
- `gitmap/cmd/probe.go` — `runProbe` + helpers (all under 15-line limit)
- `gitmap/cmd/scan.go` — `tagReposWithScanFolder` helper added
- `gitmap/constants/constants_probe.go` — SQL, error messages, CLI tokens
