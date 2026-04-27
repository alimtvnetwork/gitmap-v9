# gitmap clone-now

Re-run `git clone` against the JSON / CSV / text artifacts produced by
`gitmap scan`, honoring the recorded folder structure and a user-
selected SSH/HTTPS mode.

## Synopsis

```
gitmap clone-now <file>                            # dry-run (default)
gitmap clone-now <file> --execute                  # actually clone
gitmap clone-now <file> --mode ssh --execute       # use SSH URLs
gitmap cnow     <file> --execute                   # short alias
gitmap relclone <file> --execute                   # explicit "re-clone" verb
gitmap rc       <file> --execute                   # short alias of relclone
```

## Arguments

| Argument | Required | Description |
|---|---|---|
| `<file>` | yes | Path to a `.json`, `.csv`, or `.txt` file produced by `gitmap scan` (typically under `.gitmap/output/`). |

## Flags

| Flag | Default | Description |
|---|---|---|
| `--execute` | off | Actually run `git clone`. Without this flag, only the dry-run plan is printed. |
| `--quiet` | off | Suppress per-row progress lines. The end-of-batch summary still prints. |
| `--mode` | `https` | URL mode to clone with: `https` or `ssh`. Falls back to the other mode if the preferred URL is missing on a row. |
| `--format` | (auto) | Force input format: `json`, `csv`, or `text`. Default: detected from the file extension. |
| `--cwd` | (current dir) | Working directory each `git clone` runs in. Useful for re-creating a tree under a fresh root. |
| `--output <mode>` | (off) | Per-repo summary format. `terminal` = standardized branch/from/to/command block on **stdout**, streamed immediately before each row's `git clone`. Git progress and the batch summary stay on **stderr**. Empty (default) keeps the legacy output. |
| `--help` | off | Print this help and exit. |

## Output streams (`--output terminal`)

| Stream | Content |
|---|---|
| **stdout** | One `RepoTermBlock` per row (index, name, branch + source, original URL, target URL, exact `git clone` command). Streamed: each block prints right before that row's clone starts. Dry-run prints all blocks upfront (no clone to interleave with). |
| **stderr** | `git clone` progress, the `[i/N] status url -> dest` per-row line, the final summary, and any warnings. |

Redirect example: `gitmap clone-now repos.json --execute --output terminal > previews.txt 2> progress.log`.

## Input formats

`clone-now` consumes the same files `gitmap scan` writes. Auto-detect
maps the file extension as follows; anything else exits with a clear
"unsupported file extension" error -- pass `--format` to override.

| Extension | Parser | Notes |
|---|---|---|
| `.json` | JSON | Array of `ScanRecord` objects (see fields below). |
| `.csv`  | CSV  | Header row + one data row per repo. |
| `.txt`  | text | One `git clone <url> [dest]` line per repo; `#` comments and blank lines ignored. Branch is **not** preserved -- use JSON or CSV if you need branch pinning. |

### JSON / CSV fields

JSON keys are camelCase; CSV columns share the same names (10-column
layout produced by `gitmap scan`). A row is **clonable** when at
least one of `httpsUrl` / `sshUrl` is set.

| Field | Required | Used for | Notes |
|---|---|---|---|
| `repoName` | recommended | display, dest fallback | Shown in progress / summary lines. |
| `httpsUrl` | one of two | `--mode https` clone URL | Falls back to `sshUrl` if empty. |
| `sshUrl`   | one of two | `--mode ssh` clone URL   | Falls back to `httpsUrl` if empty. |
| `branch` | optional | `git clone -b <branch>` | Empty = clone default branch. |
| `relativePath` | recommended | clone destination (relative to `--cwd`) | If empty, derived from URL basename (sans `.git`). |
| `branchSource`, `absolutePath`, `cloneInstruction`, `notes`, `depth` | informational | display / provenance only | Read for round-trip fidelity but **not** acted on by the executor. |

### Text-format line shape

```
git clone [<flags>] <url> [<dest>]
```

`-b <branch>` flags are stripped (text format does not preserve
branch). `<dest>` falls back to the URL basename when omitted.

## Behavior

- **Folder structure preserved**: each row clones into its `relativePath` (relative to `--cwd`), mirroring the original layout.
- **Mode selection**: `--mode https`/`ssh` picks the URL column; falls back to the other when the preferred one is empty.
- **Idempotent**: an existing non-empty destination is reported `skipped`, not re-cloned. Re-running the same input is safe.
- **Sequential**: rows clone in input order. Parallel fan-out is a future addition.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Dry-run completed successfully, or every row finished `ok` / `skipped` on `--execute`. |
| `1` | File open / parse error, or any row failed during `--execute`. |
| `2` | Bad CLI usage (missing `<file>`, invalid `--mode`, invalid `--format`). |

## Examples

Dry-run a previous scan:

```
gitmap clone-now .gitmap/output/repos.json
```

Re-clone everything under a mirror root using SSH:

```
gitmap clone-now .gitmap/output/repos.csv --mode ssh --cwd ./mirror --execute
```

Force the JSON parser on a file with a non-standard extension
(auto-detect would otherwise reject `.list`):

```
gitmap clone-now my-repos.list --format json --execute
```
