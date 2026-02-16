package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeDir returns the path to the Claude projects directory.
func ClaudeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "projects")
}

// Search finds sessions matching the given term across all projects.
// It searches summary, firstPrompt (from index files) and customTitle
// (from JSONL files). Case-insensitive substring matching.
func Search(term string) ([]Session, error) {
	base := ClaudeDir()
	lower := strings.ToLower(term)

	// Map of sessionID -> Session for deduplication.
	found := make(map[string]Session)

	// 1. Search index files for summary and firstPrompt matches.
	indexes, err := filepath.Glob(filepath.Join(base, "*", "sessions-index.json"))
	if err != nil {
		return nil, fmt.Errorf("globbing index files: %w", err)
	}

	for _, idxPath := range indexes {
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

	for _, jpath := range jsonlFiles {
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

// findCustomTitle scans a JSONL file for a custom-title entry.
// Returns the title and session ID, or empty strings if not found.
func findCustomTitle(path string) (string, string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

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
			return ct.CustomTitle, ct.SessionID
		}
	}
	return "", ""
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
