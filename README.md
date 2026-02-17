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

### Interactive (TUI)

Launch the home menu to choose a mode:

```sh
clsm
```

This opens an interactive menu where you can select **Browse** or **Delete**.

### Browse

Browse all projects and drill into their sessions:

```sh
clsm browse
```

Navigate the project list, press `enter`/`l` to open a project's sessions, use `/` to filter at any level, and `r` to rename a session.

### Delete

Search and delete sessions interactively:

```sh
clsm delete
```

Or pass a search term for non-interactive CLI mode:

```sh
clsm delete "stow"
```

This searches across session summaries, first prompts, and custom titles (case-insensitive), then prompts for confirmation before deleting.

## Key Bindings

Vim-style keybindings throughout.

### Navigation

| Key | Action |
|---|---|
| `j` / `k` | Navigate up/down |
| `g` / `G` | Jump to top/bottom |
| `ctrl+u` / `ctrl+d` | Half page up/down (browse) |
| `enter` / `l` | Open / select |
| `esc` / `h` | Back |
| `q` | Quit / back |
| `/` | Filter (browse) / search (delete) |

### Browse mode (sessions)

| Key | Action |
|---|---|
| `r` | Rename session |

### Delete mode

| Key | Action |
|---|---|
| `space` | Toggle selection |
| `a` / `A` | Select all / deselect all |
| `d` / `enter` | Delete selected |
| `y` / `n` | Confirm / cancel deletion |

## How It Works

Sessions are found by scanning `~/.claude/projects/`:

1. **Index files** (`sessions-index.json`) — matches against `summary` and `firstPrompt` fields
2. **JSONL files** — scans for `custom-title` entries and matches against the `customTitle` field

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
│   │   └── delete.go                # Delete subcommand (CLI + TUI)
│   └── tui/
│       ├── theme/
│       │   └── theme.go             # Adaptive color theme (light/dark)
│       ├── home/
│       │   └── model.go             # Home menu (mode picker)
│       ├── browse/
│       │   ├── model.go             # Browse TUI (projects + sessions)
│       │   ├── update.go            # Navigation, filtering, rename
│       │   └── keys.go              # Key bindings
│       └── delete/
│           ├── model.go             # Delete TUI with progress bar
│           ├── update.go            # Search, select, confirm, delete flow
│           └── keys.go              # Key bindings
```

---

🤖 Built in collaboration with [Claude](https://claude.ai).
