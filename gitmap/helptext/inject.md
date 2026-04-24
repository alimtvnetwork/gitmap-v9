# gitmap inject (alias: inj)

Inject an existing on-disk folder into your tooling: register with
GitHub Desktop, open in VS Code, and (when a remote origin exists)
record it in the gitmap database so it appears in `cd`, `list`, etc.

## Usage

    gitmap inject              # inject the current working directory
    gitmap inject <folder>     # inject the given folder
    gitmap inj   <folder>      # short alias

`<folder>` accepts absolute, relative, or `~`-prefixed paths.

## What happens

1. **Database**: if `git remote get-url origin` succeeds, the repo is
   upserted into SQLite (HTTPS or SSH URL captured automatically).
   Local-only folders skip this step silently.
2. **GitHub Desktop**: the folder is registered. Non-repo folders are
   silently ignored by Desktop.
3. **VS Code**: opens the folder in a new window when VS Code is on
   PATH; prints a warning otherwise (no exit).
4. **Shell handoff**: the parent shell `cd`s into the injected folder
   (same UX as `clone`, `cn`, and `cd`).

## Examples

    cd ~/dev/some-repo
    gitmap inject
      → registers cwd, opens in VS Code, cds you back in.

    gitmap inject ~/dev/macro-ahk-v11
      → resolves the path, registers, opens.

    gitmap inj ../sibling-repo
      → relative paths work; resolved against cwd.

## Notes

- No `.git/` required — VS Code happily opens any folder, and Desktop
  silently skips non-repos. If you `inject` a plain folder, you'll
  see a "no remote configured, skipping database" line, then Desktop
  + VS Code proceed normally.
- For a fresh clone instead of injecting an existing folder, use
  `gitmap clone <url>`.
