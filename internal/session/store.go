package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ClaudeDir returns the path to the Claude projects directory.
func ClaudeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "projects")
}

// SearchProgress reports the current state of a search operation.
type SearchProgress struct {
	Phase   string  // "indexes" or "sessions"
	Current int     // files processed so far
	Total   int     // total files to process
	Percent float64 // 0.0 to 1.0
}

// LoadProgress reports the current state of a project loading operation.
type LoadProgress struct {
	Current int
	Total   int
	Percent float64
}

// Search finds sessions matching the given term across all projects.
// It searches summary, firstPrompt (from index files) and customTitle
// (from JSONL files). Case-insensitive substring matching.
func Search(term string) ([]Session, error) {
	results, err := SearchWithProgress(term, nil)
	return results, err
}

// SearchWithProgress is like Search but sends progress updates to the
// provided channel. The channel is closed when the search completes.
// The channel may be nil to skip progress reporting.
func SearchWithProgress(term string, progress chan<- SearchProgress) ([]Session, error) {
	if progress != nil {
		defer close(progress)
	}
	base := ClaudeDir()
	lower := strings.ToLower(term)

	// Map of sessionID -> Session for deduplication.
	found := make(map[string]Session)

	report := func(p SearchProgress) {
		if progress != nil {
			progress <- p
		}
	}

	// 1. Search index files for summary and firstPrompt matches.
	indexes, err := filepath.Glob(filepath.Join(base, "*", "sessions-index.json"))
	if err != nil {
		return nil, fmt.Errorf("globbing index files: %w", err)
	}

	for i, idxPath := range indexes {
		report(SearchProgress{
			Phase:   "indexes",
			Current: i + 1,
			Total:   len(indexes),
			Percent: float64(i+1) / float64(len(indexes)) * 0.3, // indexes = 0-30%
		})

		projectDir := filepath.Base(filepath.Dir(idxPath))

		data, err := os.ReadFile(idxPath)
		if err != nil {
			continue
		}

		var idx IndexFile
		if err := json.Unmarshal(data, &idx); err != nil {
			continue
		}

		for _, entry := range idx.Entries {
			var matchSource, matchValue string

			if strings.Contains(strings.ToLower(entry.Summary), lower) {
				matchSource = "summary"
				matchValue = entry.Summary
			} else {
				continue
			}

			found[entry.SessionID] = Session{
				SessionID:   entry.SessionID,
				Project:     projectDir,
				ProjectPath: entry.ProjectPath,
				FullPath:    entry.FullPath,
				Summary:     entry.Summary,
				FirstPrompt: entry.FirstPrompt,
				MatchSource: matchSource,
				MatchValue:  matchValue,
				Created:     entry.Created,
				Modified:    entry.Modified,
				MsgCount:    entry.MessageCount,
				GitBranch:   entry.GitBranch,
			}
		}
	}

	// 2. Scan JSONL files for custom-title matches.
	jsonlFiles, err := filepath.Glob(filepath.Join(base, "*", "*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("globbing jsonl files: %w", err)
	}

	for i, jpath := range jsonlFiles {
		report(SearchProgress{
			Phase:   "sessions",
			Current: i + 1,
			Total:   len(jsonlFiles),
			Percent: 0.3 + float64(i+1)/float64(len(jsonlFiles))*0.7, // sessions = 30-100%
		})

		title, sessionID := findCustomTitle(jpath)
		if title == "" {
			continue
		}
		if !strings.Contains(strings.ToLower(title), lower) {
			continue
		}

		projectDir := filepath.Base(filepath.Dir(jpath))

		// Custom-title takes precedence — overwrite if already found.
		existing, ok := found[sessionID]
		if ok {
			existing.CustomTitle = title
			existing.MatchSource = "custom-title"
			existing.MatchValue = title
			found[sessionID] = existing
		} else {
			// Build a session from what we know; try to fill from index.
			s := Session{
				SessionID:   sessionID,
				Project:     projectDir,
				FullPath:    jpath,
				CustomTitle: title,
				MatchSource: "custom-title",
				MatchValue:  title,
			}
			// Try to enrich from the project's index file.
			enrichFromIndex(&s, filepath.Dir(jpath))
			found[sessionID] = s
		}
	}

	results := make([]Session, 0, len(found))
	for _, s := range found {
		results = append(results, s)
	}
	return results, nil
}

// findCustomTitle scans a JSONL file for custom-title entries and returns
// the last one (most recent rename). Returns empty strings if not found.
func findCustomTitle(path string) (string, string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var lastTitle, lastSessionID string
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, `"custom-title"`) {
			continue
		}
		var ct CustomTitle
		if err := json.Unmarshal([]byte(line), &ct); err != nil {
			continue
		}
		if ct.Type == "custom-title" && ct.CustomTitle != "" {
			lastTitle = ct.CustomTitle
			lastSessionID = ct.SessionID
		}
	}
	return lastTitle, lastSessionID
}

// enrichFromIndex fills in missing Session fields from the project's index file.
func enrichFromIndex(s *Session, projectDir string) {
	idxPath := filepath.Join(projectDir, "sessions-index.json")
	data, err := os.ReadFile(idxPath)
	if err != nil {
		return
	}

	var idx IndexFile
	if err := json.Unmarshal(data, &idx); err != nil {
		return
	}

	for _, entry := range idx.Entries {
		if entry.SessionID == s.SessionID {
			s.ProjectPath = entry.ProjectPath
			s.Summary = entry.Summary
			s.FirstPrompt = entry.FirstPrompt
			s.Created = entry.Created
			s.Modified = entry.Modified
			s.MsgCount = entry.MessageCount
			s.GitBranch = entry.GitBranch
			if s.FullPath == "" {
				s.FullPath = entry.FullPath
			}
			return
		}
	}
}

// Delete removes the given sessions: deletes the JSONL file and removes
// the entry from the project's sessions-index.json.
func Delete(sessions []Session) []DeleteResult {
	results := make([]DeleteResult, 0, len(sessions))

	for _, s := range sessions {
		r := DeleteResult{SessionID: s.SessionID, Success: true}

		// 1. Remove the JSONL file.
		if err := os.Remove(s.FullPath); err != nil && !os.IsNotExist(err) {
			r.Success = false
			r.Error = fmt.Sprintf("removing session file: %v", err)
			results = append(results, r)
			continue
		}

		// 2. Update the index file.
		idxPath := filepath.Join(filepath.Dir(s.FullPath), "sessions-index.json")
		if err := removeFromIndex(idxPath, s.SessionID); err != nil {
			r.Success = false
			r.Error = fmt.Sprintf("updating index: %v", err)
		}

		results = append(results, r)
	}

	return results
}

// Rename sets a new custom title for a session by appending a custom-title
// entry to its JSONL file.
func Rename(s Session, newTitle string) error {
	entry := CustomTitle{
		Type:        "custom-title",
		CustomTitle: newTitle,
		SessionID:   s.SessionID,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling custom-title: %w", err)
	}

	f, err := os.OpenFile(s.FullPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening session file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append([]byte("\n"), data...)); err != nil {
		return fmt.Errorf("writing custom-title: %w", err)
	}

	return nil
}

// ListProjects returns all projects that contain sessions, sorted by most
// recently modified session.
func ListProjects() ([]Project, error) {
	return ListProjectsWithProgress(nil)
}

// ListProjectsWithProgress is like ListProjects but sends progress updates to
// the provided channel. The channel is closed when loading completes.
// The channel may be nil to skip progress reporting.
func ListProjectsWithProgress(progress chan<- LoadProgress) ([]Project, error) {
	if progress != nil {
		defer close(progress)
	}
	base := ClaudeDir()

	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("reading projects dir: %w", err)
	}

	var dirs []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		}
	}

	report := func(current, total int) {
		if progress != nil {
			progress <- LoadProgress{
				Current: current,
				Total:   total,
				Percent: float64(current) / float64(total),
			}
		}
	}

	var projects []Project
	for i, dir := range dirs {
		report(i+1, len(dirs))
		dirName := dir.Name()
		dirPath := filepath.Join(base, dirName)

		// Try to read the index file first.
		idxPath := filepath.Join(dirPath, "sessions-index.json")
		if data, err := os.ReadFile(idxPath); err == nil {
			var idx IndexFile
			if err := json.Unmarshal(data, &idx); err == nil && len(idx.Entries) > 0 {
				var projectPath, lastModified, lastPrompt string
				for _, e := range idx.Entries {
					if projectPath == "" && e.ProjectPath != "" {
						projectPath = e.ProjectPath
					}
					if e.Modified > lastModified {
						lastModified = e.Modified
						if e.Summary != "" {
							lastPrompt = e.Summary
						} else if e.FirstPrompt != "" {
							lastPrompt = e.FirstPrompt
						}
					}
				}
				if projectPath == "" {
					projectPath = decodeDirName(dirName)
				}
				projects = append(projects, Project{
					DirName:      dirName,
					Path:         projectPath,
					SessionCount: len(idx.Entries),
					LastModified: lastModified,
					LastPrompt:   lastPrompt,
				})
				continue
			}
		}

		// No index or empty — fall back to counting .jsonl files.
		jsonlFiles, _ := filepath.Glob(filepath.Join(dirPath, "*.jsonl"))
		if len(jsonlFiles) == 0 {
			continue
		}

		var lastModified time.Time
		for _, jpath := range jsonlFiles {
			info, err := os.Stat(jpath)
			if err != nil {
				continue
			}
			if info.ModTime().After(lastModified) {
				lastModified = info.ModTime()
			}
		}

		// Try to find a first prompt, starting from the newest file.
		var lastPrompt string
		// Sort by mod time descending so we check newest first.
		sort.Slice(jsonlFiles, func(a, b int) bool {
			ai, _ := os.Stat(jsonlFiles[a])
			bi, _ := os.Stat(jsonlFiles[b])
			if ai == nil || bi == nil {
				return false
			}
			return ai.ModTime().After(bi.ModTime())
		})
		for _, jpath := range jsonlFiles {
			if p := extractFirstPrompt(jpath); p != "" {
				lastPrompt = p
				break
			}
		}

		projects = append(projects, Project{
			DirName:      dirName,
			Path:         decodeDirName(dirName),
			SessionCount: len(jsonlFiles),
			LastModified: lastModified.Format(time.RFC3339),
			LastPrompt:   lastPrompt,
		})
	}

	// Sort by last modified descending.
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastModified > projects[j].LastModified
	})

	return projects, nil
}

// ListSessions returns all sessions for a given project directory,
// sorted by modified date descending. It also enriches sessions with
// custom titles from JSONL files.
func ListSessions(projectDir string) ([]Session, error) {
	base := ClaudeDir()
	projPath := filepath.Join(base, projectDir)

	// Build a map of custom titles from JSONL files.
	customTitles := make(map[string]string)
	jsonlFiles, _ := filepath.Glob(filepath.Join(projPath, "*.jsonl"))
	for _, jpath := range jsonlFiles {
		title, sessionID := findCustomTitle(jpath)
		if title != "" {
			customTitles[sessionID] = title
		}
	}

	// Try to read the index file.
	idxPath := filepath.Join(projPath, "sessions-index.json")
	if data, err := os.ReadFile(idxPath); err == nil {
		var idx IndexFile
		if err := json.Unmarshal(data, &idx); err == nil && len(idx.Entries) > 0 {
			sessions := make([]Session, 0, len(idx.Entries))
			for _, e := range idx.Entries {
				s := Session{
					SessionID:   e.SessionID,
					Project:     projectDir,
					ProjectPath: e.ProjectPath,
					FullPath:    e.FullPath,
					Summary:     e.Summary,
					FirstPrompt: e.FirstPrompt,
					Created:     e.Created,
					Modified:    e.Modified,
					MsgCount:    e.MessageCount,
					GitBranch:   e.GitBranch,
				}
				if t, ok := customTitles[e.SessionID]; ok {
					s.CustomTitle = t
				}
				sessions = append(sessions, s)
			}

			sort.Slice(sessions, func(i, j int) bool {
				ti, _ := time.Parse(time.RFC3339, sessions[i].Modified)
				tj, _ := time.Parse(time.RFC3339, sessions[j].Modified)
				return ti.After(tj)
			})

			return sessions, nil
		}
	}

	// No index — build sessions from .jsonl files.
	sessions := make([]Session, 0, len(jsonlFiles))
	for _, jpath := range jsonlFiles {
		fname := filepath.Base(jpath)
		sessionID := strings.TrimSuffix(fname, ".jsonl")

		var modified string
		if info, err := os.Stat(jpath); err == nil {
			modified = info.ModTime().Format(time.RFC3339)
		}

		s := Session{
			SessionID:   sessionID,
			Project:     projectDir,
			ProjectPath: decodeDirName(projectDir),
			FullPath:    jpath,
			Modified:    modified,
		}
		if t, ok := customTitles[sessionID]; ok {
			s.CustomTitle = t
		}
		sessions = append(sessions, s)
	}

	sort.Slice(sessions, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, sessions[i].Modified)
		tj, _ := time.Parse(time.RFC3339, sessions[j].Modified)
		return ti.After(tj)
	})

	return sessions, nil
}

// extractFirstPrompt reads a JSONL session file and returns the content of
// the first user message. Returns empty string if none found.
func extractFirstPrompt(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, `"type":"user"`) {
			continue
		}
		var entry struct {
			Type    string `json:"type"`
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Type == "user" && entry.Message.Content != "" {
			return entry.Message.Content
		}
	}
	return ""
}

// decodeDirName converts an encoded project directory name back to a path.
// e.g. "-Users-barryhall-Dev-code" -> "/Users/barryhall/Dev/code"
// Claude Code encodes both "/" and "." as "-", so "--" represents "/."
// (a hidden directory like .config, .claude). This is a heuristic and
// can't distinguish hyphens in real directory names (e.g. "my-project").
func decodeDirName(name string) string {
	if len(name) == 0 {
		return ""
	}
	s := name[1:]
	// "--" is "/." (hidden directory prefix).
	s = strings.ReplaceAll(s, "--", "/.")
	s = strings.ReplaceAll(s, "-", "/")
	return "/" + s
}

// removeFromIndex reads the index file, filters out the given session ID,
// and writes it back.
func removeFromIndex(idxPath, sessionID string) error {
	data, err := os.ReadFile(idxPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no index to update
		}
		return err
	}

	var idx IndexFile
	if err := json.Unmarshal(data, &idx); err != nil {
		return fmt.Errorf("parsing index: %w", err)
	}

	filtered := make([]IndexEntry, 0, len(idx.Entries))
	for _, e := range idx.Entries {
		if e.SessionID != sessionID {
			filtered = append(filtered, e)
		}
	}
	idx.Entries = filtered

	out, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling index: %w", err)
	}

	return os.WriteFile(idxPath, out, 0644)
}
