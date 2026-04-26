# gitmap startup-add

Create a Linux/Unix XDG autostart entry that runs gitmap (or any
command) at login. The created `.desktop` file is tagged with
`X-Gitmap-Managed=true` so `startup-list` and `startup-remove` can
safely manage it without touching third-party autostart files.

## Alias

sa

## Usage

    gitmap startup-add --name <id> [--exec <path>] [--display-name <s>]
                       [--comment <s>] [--no-display] [--force]

## Flags

| Flag | Required | Description |
|------|----------|-------------|
| --name           | yes | Logical name; filename becomes `gitmap-<name>.desktop` |
| --exec           | no  | Command to run at login (default: path to running gitmap binary) |
| --display-name   | no  | Override the `Name=` field shown in session managers |
| --comment        | no  | Optional `Comment=` text |
| --no-display     | no  | Set `NoDisplay=true` (hide from app menus, still autostarts) |
| --force          | no  | Overwrite an existing **gitmap-managed** entry (never overwrites third-party files) |

## Prerequisites

- Linux or other Unix with `~/.config/autostart` (XDG-compliant).
- macOS uses LaunchAgents — not handled here.
- On Windows, the command exits with the standard "unsupported OS"
  message.

## Safety

- Refuses to overwrite a `.desktop` file that does NOT carry the
  `X-Gitmap-Managed=true` marker, even with `--force`.
- Names containing path separators (`/`, `\`) or NUL are rejected
  before any I/O.
- Atomic write (temp file + rename) so a crash mid-write cannot
  leave a half-written file the next login session would execute.

## Examples

### Example 1: Add gitmap itself with default args

    gitmap startup-add --name watch --exec "$(command -v gitmap) watch"

**Output:**

    ✓ Created gitmap-managed autostart entry: /home/me/.config/autostart/gitmap-watch.desktop

### Example 2: Re-run is idempotent

    gitmap startup-add --name watch --exec "$(command -v gitmap) watch"

**Output:**

      (exists) gitmap-managed entry already at /home/me/.config/autostart/gitmap-watch.desktop — pass --force to overwrite

### Example 3: Update an existing entry

    gitmap startup-add --name watch \
      --exec "$(command -v gitmap) watch --quiet" --force

**Output:**

    ✓ Overwrote gitmap-managed autostart entry: /home/me/.config/autostart/gitmap-watch.desktop

## See Also

- [startup-list](startup-list.md) — List entries gitmap created
- [startup-remove](startup-remove.md) — Delete a gitmap-managed entry
