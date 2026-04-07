package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// PlansDir returns the path to the Claude plans directory.
func PlansDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "plans")
}

// ListPlans returns all plan files sorted by modification time descending.
func ListPlans() ([]Plan, error) {
	dir := PlansDir()

	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("globbing plan files: %w", err)
	}

	var plans []Plan
	for _, f := range files {
		p, err := ReadPlan(f)
		if err != nil {
			continue
		}
		plans = append(plans, p)
	}

	sort.Slice(plans, func(i, j int) bool {
		return plans[i].ModTime > plans[j].ModTime
	})

	return plans, nil
}

// ReadPlan reads and parses a plan file, extracting title, context, and project hint.
func ReadPlan(path string) (Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Plan{}, err
	}

	content := string(data)
	info, _ := os.Stat(path)

	p := Plan{
		FileName: filepath.Base(path),
		FullPath: path,
	}

	if info != nil {
		p.ModTime = info.ModTime().Format(time.RFC3339)
		p.Size = info.Size()
	}

	p.Title = extractTitle(content)
	p.Context = extractContext(content)
	p.ProjectHint = extractProjectHint(content)

	if p.Title == "" {
		p.Title = strings.TrimSuffix(p.FileName, ".md")
	}

	return p, nil
}

// Delete removes the given plan files.
func Delete(plans []Plan) []DeleteResult {
	results := make([]DeleteResult, 0, len(plans))
	for _, p := range plans {
		r := DeleteResult{FileName: p.FileName, Success: true}
		if err := os.Remove(p.FullPath); err != nil && !os.IsNotExist(err) {
			r.Success = false
			r.Error = fmt.Sprintf("removing file: %v", err)
		}
		results = append(results, r)
	}
	return results
}

// extractTitle returns the text of the first # heading.
func extractTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

// extractContext returns the first paragraph under a ## Context, ## Overview,
// or ## Summary heading. Returns a single line, truncated.
func extractContext(content string) string {
	lines := strings.Split(content, "\n")
	inSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") {
			heading := strings.ToLower(strings.TrimPrefix(trimmed, "## "))
			if heading == "context" || heading == "overview" || heading == "summary" ||
				heading == "executive summary" || heading == "overall assessment" {
				inSection = true
				continue
			}
			if inSection {
				break // hit next heading
			}
		}

		if inSection && trimmed != "" && !strings.HasPrefix(trimmed, "---") {
			// Return first non-empty line as the context summary.
			if len(trimmed) > 200 {
				return trimmed[:197] + "..."
			}
			return trimmed
		}
	}

	return ""
}

var (
	// Match absolute paths like /Users/someone/Dev/project
	absPathRe = regexp.MustCompile(`/Users/\w+/[\w/.-]+`)
	// Match Go module paths like github.com/org/repo or charm.land/pkg
	moduleRe = regexp.MustCompile(`(?:github\.com|charm\.land)/[\w.-]+/[\w.-]+`)
)

// extractProjectHint scans content for project-identifying paths or module names.
func extractProjectHint(content string) string {
	// Try absolute paths first — most specific.
	if matches := absPathRe.FindAllString(content, 10); len(matches) > 0 {
		// Find the most common path prefix (likely the project root).
		counts := make(map[string]int)
		for _, m := range matches {
			// Trim to a reasonable project root depth.
			parts := strings.Split(m, "/")
			// /Users/name/Dev/area/project = 6 parts is typical
			if len(parts) > 6 {
				parts = parts[:6]
			}
			root := strings.Join(parts, "/")
			counts[root]++
		}
		var best string
		var bestCount int
		for path, count := range counts {
			if count > bestCount {
				best = path
				bestCount = count
			}
		}
		return shortenPath(best)
	}

	// Try Go module paths.
	if matches := moduleRe.FindAllString(content, 5); len(matches) > 0 {
		return matches[0]
	}

	return ""
}

func shortenPath(path string) string {
	home, _ := strings.CutPrefix(path, "/Users/")
	if home != path {
		parts := strings.SplitN(home, "/", 2)
		if len(parts) == 2 {
			return "~/" + parts[1]
		}
		return "~"
	}
	return path
}
