```
  ██████╗ ██╗      ███████╗ ███╗   ███╗
 ██╔════╝ ██║      ██╔════╝ ████╗ ████║
 ██║      ██║      ███████╗ ██╔████╔██║
 ██║      ██║      ╚════██║ ██║╚██╔╝██║
 ╚██████╗ ███████╗ ███████║ ██║ ╚═╝ ██║
  ╚═════╝ ╚══════╝ ╚══════╝ ╚═╝     ╚═╝
```

**Claude Session Manager** — a CLI/TUI tool for managing Claude Code sessions.

Claude Code stores session data in `~/.claude/projects/` as JSONL files. `clsm` provides a fast, standalone way to browse, search, and delete those sessions from the terminal.

## Install

```sh
go install github.com/baz-sh/clsm@latest
```

Or build from source:

```sh
git clone https://github.com/baz-sh/clsm.git
cd clsm
go build -o clsm .
```

## Usage

```sh
clsm
```

This opens an interactive menu with three options:

- **Projects** — browse projects, drill into their sessions
- **Sessions** — browse all sessions across all projects
- **Search** — search sessions by summary or custom title

From any session list you can navigate with `j`/`k`, rename with `r`, multi-select with `space`, and delete selected sessions with `d` (with confirmation).

Use `/` to filter at any level.

### CLI Delete

For scripting or quick one-off deletions:

```sh
clsm delete "stow"
```

Searches across session summaries and custom titles (case-insensitive), shows matches, and prompts for confirmation before deleting.

## Key Bindings

Vim-style keybindings throughout.

### Navigation

| Key | Action |
|---|---|
| `j` / `k` | Navigate up/down |
| `g` / `G` | Jump to top/bottom |
| `ctrl+u` / `ctrl+d` | Half page up/down |
| `enter` / `l` | Open / select |
| `esc` / `h` | Back |
| `q` | Quit |
| `/` | Filter |

### Sessions

| Key | Action |
|---|---|
| `space` | Toggle selection |
| `a` / `A` | Select all / deselect all |
| `r` | Rename session |
| `d` | Delete selected |
| `y` / `n` | Confirm / cancel deletion |

## How It Works

Sessions are found by scanning `~/.claude/projects/`:

1. **Index files** (`sessions-index.json`) — reads session metadata (summary, message count, timestamps)
2. **JSONL files** — scans for `custom-title` entries and enriches missing data (message counts, first prompts) directly from session files

When deleting, `clsm` removes the `.jsonl` session file and removes the corresponding entry from the project's `sessions-index.json`.

When renaming, `clsm` appends a new `custom-title` entry to the session's JSONL file — the same mechanism Claude Code uses internally.

The TUI adapts colors automatically to light and dark terminal backgrounds.

## Project Structure

```
clsm/
├── main.go                          # Entry point
├── internal/
│   ├── session/
│   │   ├── types.go                 # Domain types (Session, Project, etc.)
│   │   └── store.go                 # Search, delete, rename, list projects/sessions
│   ├── cmd/
│   │   ├── root.go                  # Root command + home menu launcher
│   │   ├── browse.go                # Browse subcommand
│   │   └── delete.go                # Delete subcommand (CLI only)
│   └── tui/
│       ├── theme/
│       │   └── theme.go             # Adaptive color theme (light/dark)
│       ├── home/
│       │   └── model.go             # Home menu (Projects/Sessions/Search)
│       ├── browse/
│       │   ├── model.go             # Browse TUI (projects, sessions, search, delete)
│       │   ├── update.go            # Navigation, filtering, rename, multi-select, delete
│       │   └── keys.go              # Key bindings
│       └── delete/
│           ├── model.go             # Delete TUI (unused, kept for reference)
│           ├── update.go            # Search, select, confirm, delete flow
│           └── keys.go              # Key bindings
```

---

🤖 Built in collaboration with [Claude](https://claude.ai).
