package memory

import (
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

// ListProjects returns all projects that contain memory directories
// with at least one .md file, sorted by most recently modified memory.
func ListProjects() ([]MemoryProject, error) {
	return ListProjectsWithProgress(nil)
}

// ListProjectsWithProgress is like ListProjects but sends progress updates.
// The channel is closed when loading completes. May be nil to skip progress.
func ListProjectsWithProgress(progress chan<- LoadProgress) ([]MemoryProject, error) {
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

	var projects []MemoryProject
	for i, dir := range dirs {
		report(i+1, len(dirs))
		dirName := dir.Name()
		memDir := filepath.Join(base, dirName, "memory")

		info, err := os.Stat(memDir)
		if err != nil || !info.IsDir() {
			continue
		}

		memFiles, _ := filepath.Glob(filepath.Join(memDir, "*.md"))

		var count int
		var hasIndex bool
		var lastMod time.Time

		for _, f := range memFiles {
			name := filepath.Base(f)
			if name == "MEMORY.md" {
				hasIndex = true
				continue
			}
			count++
			if fi, err := os.Stat(f); err == nil {
				if fi.ModTime().After(lastMod) {
					lastMod = fi.ModTime()
				}
			}
		}

		if count == 0 && !hasIndex {
			continue
		}

		var lastModStr string
		if !lastMod.IsZero() {
			lastModStr = lastMod.Format(time.RFC3339)
		}

		projects = append(projects, MemoryProject{
			DirName:      dirName,
			Path:         decodeDirName(dirName),
			MemoryCount:  count,
			HasIndex:     hasIndex,
			LastModified: lastModStr,
		})
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastModified > projects[j].LastModified
	})

	return projects, nil
}

// ListMemories returns all memory files for a given project directory,
// sorted by modification time descending. Excludes MEMORY.md.
func ListMemories(projectDir string) ([]Memory, error) {
	base := ClaudeDir()
	memDir := filepath.Join(base, projectDir, "memory")

	files, err := filepath.Glob(filepath.Join(memDir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("globbing memory files: %w", err)
	}

	var memories []Memory
	for _, f := range files {
		if filepath.Base(f) == "MEMORY.md" {
			continue
		}
		m, err := ReadMemory(f)
		if err != nil {
			continue
		}
		m.ProjectDir = projectDir
		m.ProjectPath = decodeDirName(projectDir)
		memories = append(memories, m)
	}

	sort.Slice(memories, func(i, j int) bool {
		return memories[i].ModTime > memories[j].ModTime
	})

	return memories, nil
}

// ReadMemory reads and parses a single memory file, including YAML frontmatter.
func ReadMemory(path string) (Memory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Memory{}, err
	}
	content := string(data)

	m := Memory{
		FullPath: path,
		FileName: filepath.Base(path),
	}

	if info, err := os.Stat(path); err == nil {
		m.ModTime = info.ModTime().Format(time.RFC3339)
	}

	// Parse YAML frontmatter between --- delimiters.
	if strings.HasPrefix(content, "---\n") {
		end := strings.Index(content[4:], "\n---\n")
		if end >= 0 {
			frontmatter := content[4 : 4+end]
			m.Content = strings.TrimSpace(content[4+end+5:])
			for _, line := range strings.Split(frontmatter, "\n") {
				k, v, ok := strings.Cut(line, ": ")
				if !ok {
					continue
				}
				v = strings.TrimSpace(v)
				switch strings.TrimSpace(k) {
				case "name":
					m.Name = v
				case "description":
					m.Description = v
				case "type":
					m.Type = v
				}
			}
		} else {
			m.Content = content
		}
	} else {
		m.Content = content
	}

	// Fallback: use filename (without extension) as name.
	if m.Name == "" {
		m.Name = strings.TrimSuffix(m.FileName, ".md")
	}

	return m, nil
}

// Delete removes the given memory files and updates the MEMORY.md index
// in each affected project directory.
func Delete(memories []Memory) []DeleteResult {
	results := make([]DeleteResult, 0, len(memories))

	// Group by directory so we update each MEMORY.md once.
	byDir := make(map[string][]string) // dir -> filenames

	for _, m := range memories {
		r := DeleteResult{FileName: m.FileName, Success: true}

		if err := os.Remove(m.FullPath); err != nil && !os.IsNotExist(err) {
			r.Success = false
			r.Error = fmt.Sprintf("removing file: %v", err)
			results = append(results, r)
			continue
		}

		dir := filepath.Dir(m.FullPath)
		byDir[dir] = append(byDir[dir], m.FileName)
		results = append(results, r)
	}

	// Update MEMORY.md in each affected directory.
	for dir, filenames := range byDir {
		removeFromIndex(filepath.Join(dir, "MEMORY.md"), filenames)
	}

	return results
}

// removeFromIndex reads a MEMORY.md file, removes lines referencing the
// given filenames, and writes it back. Errors are silently ignored.
func removeFromIndex(indexPath string, filenames []string) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return
	}

	removed := make(map[string]bool)
	for _, f := range filenames {
		removed[f] = true
	}

	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		shouldRemove := false
		for f := range removed {
			if strings.Contains(line, "("+f+")") {
				shouldRemove = true
				break
			}
		}
		if !shouldRemove {
			kept = append(kept, line)
		}
	}

	// Trim trailing empty lines.
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}

	result := strings.Join(kept, "\n") + "\n"
	os.WriteFile(indexPath, []byte(result), 0644)
}

// decodeDirName converts an encoded project directory name back to a path.
// e.g. "-Users-barryhall-Dev-code" -> "/Users/barryhall/Dev/code"
func decodeDirName(name string) string {
	if len(name) == 0 {
		return ""
	}
	s := name[1:]
	s = strings.ReplaceAll(s, "--", "/.")
	s = strings.ReplaceAll(s, "-", "/")
	return "/" + s
}
