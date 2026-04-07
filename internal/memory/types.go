package memory

// Memory represents a single Claude memory file with parsed frontmatter.
type Memory struct {
	Name        string // from YAML frontmatter "name" field
	Description string // from YAML frontmatter "description" field
	Type        string // "user", "feedback", "project", "reference"
	Content     string // markdown body after frontmatter
	FileName    string // e.g. "feedback_github_urls.md"
	FullPath    string // absolute path to the .md file
	ProjectDir  string // encoded project directory name
	ProjectPath string // decoded project path
	ModTime     string // file modification time as RFC3339
}

// MemoryProject represents a project that has a memory directory.
type MemoryProject struct {
	DirName      string // encoded directory name
	Path         string // decoded project path
	MemoryCount  int    // number of .md files in memory/ (excluding MEMORY.md)
	HasIndex     bool   // whether MEMORY.md exists
	LastModified string // most recent memory file modification time
}

// DeleteResult tracks the outcome of deleting a single memory.
type DeleteResult struct {
	FileName string
	Success  bool
	Error    string
}

// LoadProgress reports the current state of a loading operation.
type LoadProgress struct {
	Current int
	Total   int
	Percent float64
}
