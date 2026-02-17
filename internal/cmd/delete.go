package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/baz-sh/clsm/internal/session"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <search-term>",
	Short: "Delete Claude Code sessions",
	Long: `Delete Claude Code sessions matching a search term.

Finds matches, shows them, and prompts for confirmation before deleting.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCLI(strings.Join(args, " "))
	},
}

func runCLI(term string) error {
	sessions, err := session.Search(term)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found matching:", term)
		return nil
	}

	fmt.Printf("Found %d session(s) matching %q:\n\n", len(sessions), term)
	for i, s := range sessions {
		title := s.CustomTitle
		if title == "" {
			title = s.Summary
		}
		if title == "" {
			title = s.FirstPrompt
		}
		fmt.Printf("  %d. %s\n", i+1, title)
		fmt.Printf("     Project: %s\n", s.ProjectPath)
		fmt.Printf("     Match:   %s (%s)\n", s.MatchValue, s.MatchSource)
		fmt.Printf("     Created: %s  Messages: %d\n\n", s.Created, s.MsgCount)
	}

	fmt.Print("Delete these sessions? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "y" && answer != "yes" {
		fmt.Println("Aborted.")
		return nil
	}

	results := session.Delete(sessions)
	var failed int
	for _, r := range results {
		if r.Success {
			fmt.Printf("  Deleted: %s\n", r.SessionID)
		} else {
			fmt.Printf("  Failed:  %s — %s\n", r.SessionID, r.Error)
			failed++
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d session(s) failed to delete", failed)
	}
	return nil
}
