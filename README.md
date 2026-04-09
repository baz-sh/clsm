```
  ██████╗ ██╗      ███████╗ ███╗   ███╗
 ██╔════╝ ██║      ██╔════╝ ████╗ ████║
 ██║      ██║      ███████╗ ██╔████╔██║
 ██║      ██║      ╚════██║ ██║╚██╔╝██║
 ╚██████╗ ███████╗ ███████║ ██║ ╚═╝ ██║
  ╚═════╝ ╚══════╝ ╚══════╝ ╚═╝     ╚═╝
```

**Claude Session Manager** — a CLI/TUI tool for managing Claude Code sessions, memories, and plans.

Claude Code stores session data as JSONL files, memories as markdown with YAML frontmatter, and plans as markdown files across `~/.claude/`. `clsm` provides a fast, standalone way to browse, search, and manage all of it from the terminal.

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

This opens an interactive menu with six options:

- **Projects** — browse projects and their sessions
- **Sessions** — browse all sessions across all projects
- **Search** — search sessions by summary, custom title, first prompt, or project path
- **Memories** — browse and manage Claude memories per project
- **Plans** — browse and clean up Claude plans
- **Prune** — find and delete sessions with zero messages

All views use vim-style navigation (`j`/`k`), filtering (`/`), multi-select (`space`), and delete (`d` with confirmation). Sessions can also be renamed with `r`.

## Key Bindings

Vim-style keybindings throughout.

### Navigation

| Key | Action |
|---|---|
| `j` / `k` | Navigate up/down |
| `g` / `G` | Jump to top/bottom |
| `ctrl+u` / `ctrl+d` | Half page up/down |
| `enter` / `l` / `space` | Open / select |
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
| `y` / `n` | Confirm / cancel |

### Memories

| Key | Action |
|---|---|
| `space` | Toggle selection |
| `a` / `A` | Select all / deselect all |
| `d` | Delete selected |
| `y` / `n` | Confirm / cancel |

### Plans

| Key | Action |
|---|---|
| `space` | Toggle selection |
| `a` / `A` | Select all / deselect all |
| `d` | Delete selected |
| `e` | Open in `$EDITOR` |
| `y` / `n` | Confirm / cancel |

## How It Works

### Sessions

Sessions are found by scanning `~/.claude/projects/`:

1. **Index files** (`sessions-index.json`) — reads session metadata (summary, message count, timestamps, git branch)
2. **JSONL files** — scans for `custom-title` entries and enriches missing data (message counts, first prompts) directly from session files

When deleting, `clsm` removes the `.jsonl` session file and removes the corresponding entry from the project's `sessions-index.json`.

When renaming, `clsm` appends a new `custom-title` entry to the session's JSONL file — the same mechanism Claude Code uses internally.

When pruning, `clsm` loads all sessions and deletes those with zero messages.

### Memories

Memories are markdown files with YAML frontmatter stored in `~/.claude/projects/<project>/memory/`. `clsm` parses the frontmatter (name, description, type) and renders content with syntax-highlighted markdown. When deleting a memory, the corresponding entry is also removed from the project's `MEMORY.md` index.

### Plans

Plans are markdown files stored in `~/.claude/plans/`. `clsm` extracts metadata (title from the first heading, context from overview sections, project hints from paths in the content) and renders them with syntax-highlighted markdown. Plans can be opened in `$EDITOR` with `e`.

### Theme

The TUI adapts colors automatically to light and dark terminal backgrounds.

## Project Structure

```
clsm/
├── main.go                          # Entry point
├── internal/
│   ├── session/
│   │   ├── types.go                 # Domain types (Session, Project, etc.)
│   │   └── store.go                 # Search, delete, rename, list projects/sessions
│   ├── memory/
│   │   ├── types.go                 # Memory and MemoryProject types
│   │   └── store.go                 # Memory file I/O, frontmatter parsing, deletion
│   ├── plan/
│   │   ├── types.go                 # Plan types
│   │   └── store.go                 # Plan file I/O, metadata extraction, deletion
│   ├── cmd/
│   │   ├── root.go                  # Root command + home menu launcher
│   │   ├── browse.go                # Browse subcommand
│   │   ├── delete.go                # Delete subcommand (CLI only)
│   │   ├── memories.go              # Memories subcommand
│   │   └── plans.go                 # Plans subcommand
│   └── tui/
│       ├── theme/
│       │   └── theme.go             # Adaptive color theme (light/dark)
│       ├── home/
│       │   └── model.go             # Home menu
│       ├── browse/
│       │   ├── model.go             # Session browser TUI
│       │   ├── update.go            # Navigation, filtering, rename, multi-select, delete
│       │   └── keys.go              # Key bindings
│       ├── memorybrowse/
│       │   ├── model.go             # Memory browser TUI
│       │   ├── update.go            # Memory navigation, viewing, deletion
│       │   └── keys.go              # Key bindings
│       └── planbrowse/
│           ├── model.go             # Plan browser TUI
│           ├── update.go            # Plan navigation, viewing, deletion
│           └── keys.go              # Key bindings
```

---

🤖 Built in collaboration with [Claude](https://claude.ai).
