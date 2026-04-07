package plan

// Plan represents a Claude plan file with extracted metadata.
type Plan struct {
	FileName    string // e.g. "fluffy-coalescing-giraffe.md"
	FullPath    string
	Title       string // from first # heading
	Context     string // first paragraph under ## Context/Overview/Summary
	ProjectHint string // heuristic: best-guess project path from content
	ModTime     string // RFC3339
	Size        int64  // bytes
}

// DeleteResult tracks the outcome of deleting a single plan.
type DeleteResult struct {
	FileName string
	Success  bool
	Error    string
}
