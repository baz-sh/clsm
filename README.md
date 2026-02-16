```
  ██████╗ ██╗      ███████╗ ███╗   ███╗
 ██╔════╝ ██║      ██╔════╝ ████╗ ████║
 ██║      ██║      ███████╗ ██╔████╔██║
 ██║      ██║      ╚════██║ ██║╚██╔╝██║
 ╚██████╗ ███████╗ ███████║ ██║ ╚═╝ ██║
  ╚═════╝ ╚══════╝ ╚══════╝ ╚═╝     ╚═╝
```

**Claude Session Manager** — a CLI/TUI tool for managing Claude Code sessions.

Claude Code stores session data in `~/.claude/projects/` as JSONL files. `clsm` provides a fast, standalone way to search, inspect, and delete those sessions from the terminal.

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

### CLI mode

Pass a search term to find and delete matching sessions non-interactively:

```sh
clsm delete "stow"
```

This searches across session summaries, first prompts, and custom titles (case-insensitive), then prompts for confirmation before deleting.

### TUI mode

Launch the interactive terminal UI:

```sh
clsm delete
```

This opens a full-screen interface with search, multi-select, and confirmation phases.

## TUI Key Bindings

| Key | Action |
|---|---|
| `j` / `k` | Navigate up/down |
| `g` / `G` | Jump to top/bottom |
| `space` | Toggle selection |
| `a` / `A` | Select all / deselect all |
| `d` / `enter` | Delete selected |
| `/` | New search |
| `esc` | Back |
| `q` | Quit |

## How It Works

Sessions are found by scanning `~/.claude/projects/`:

1. **Index files** (`sessions-index.json`) — matches against `summary` and `firstPrompt` fields
2. **JSONL files** — scans for `custom-title` entries and matches against the `customTitle` field

When deleting, `clsm` removes the `.jsonl` session file and removes the corresponding entry from the project's `sessions-index.json`.

## Project Structure

```
clsm/
├── main.go                    # Entry point
├── internal/
│   ├── session/
│   │   ├── types.go           # Domain types
│   │   └── store.go           # Search and delete logic
│   ├── cmd/
│   │   ├── root.go            # Root Cobra command
│   │   └── delete.go          # Delete subcommand (CLI + TUI)
│   └── tui/
│       └── delete/
│           ├── model.go       # Bubble Tea model, init, view
│           ├── update.go      # Update logic, async commands
│           └── keys.go        # Key bindings
```

---

🤖 Built in collaboration with [Claude](https://claude.ai).
