package session

// Session represents a Claude Code session with metadata from
// both the index file and the JSONL session file.
type Session struct {
	SessionID   string
	Project     string // project directory name
	ProjectPath string // original project path (e.g. /Users/<USERNAME>/.config)
	FullPath    string // absolute path to .jsonl file
	Summary     string
	FirstPrompt string
	CustomTitle string // from JSONL custom-title entry (if any)
	MatchSource string // "custom-title", "summary", or "firstPrompt"
	MatchValue  string // the value that matched the search
	Created     string
	Modified    string
	MsgCount    int
	GitBranch   string
}

// IndexFile represents the sessions-index.json structure.
type IndexFile struct {
	Version int          `json:"version"`
	Entries []IndexEntry `json:"entries"`
}

// IndexEntry represents a single session in the index.
type IndexEntry struct {
	SessionID    string `json:"sessionId"`
	FullPath     string `json:"fullPath"`
	FileMtime    int64  `json:"fileMtime"`
	FirstPrompt  string `json:"firstPrompt"`
	Summary      string `json:"summary"`
	MessageCount int    `json:"messageCount"`
	Created      string `json:"created"`
	Modified     string `json:"modified"`
	GitBranch    string `json:"gitBranch"`
	ProjectPath  string `json:"projectPath"`
	IsSidechain  bool   `json:"isSidechain"`
}

// CustomTitle represents a custom-title JSONL entry.
type CustomTitle struct {
	Type        string `json:"type"`
	CustomTitle string `json:"customTitle"`
	SessionID   string `json:"sessionId"`
}

// DeleteResult tracks the outcome of deleting a single session.
type DeleteResult struct {
	SessionID string
	Success   bool
	Error     string
}

// Project represents a Claude Code project directory containing sessions.
type Project struct {
	DirName      string // encoded directory name (e.g. "-Users-barryhall-Dev-code")
	Path         string // original project path (e.g. "/Users/barryhall/Dev/code")
	SessionCount int
	LastModified string // most recent session modified date
	LastPrompt   string // summary or first prompt from the most recent session
}
