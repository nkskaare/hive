package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/nkskaare/hive/internal/agent"
	"github.com/spf13/cobra"
)

var spawnCmd = &cobra.Command{
	Use:   "spawn [id] [branch]",
	Short: "Spawn a new agent sandbox",
	Long:  "Creates a git worktree, starts a Docker container, runs post-spawn hooks, and opens a terminal tab.",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var agentID, branch string

		if len(args) >= 1 {
			agentID = args[0]
		} else {
			err := huh.NewInput().
				Title("Agent ID").
				Placeholder("e.g. fix-login-bug").
				Validate(huh.ValidateNotEmpty()).
				Value(&agentID).
				Run()
			if err != nil {
				return err
			}
		}

		if len(args) >= 2 {
			branch = args[1]
		} else {
			branch = agentID
		}

		if dryRun {
			fmt.Printf("Would spawn agent %q on branch %q\n", agentID, branch)
			return nil
		}

		return agent.Spawn(cfg, agentID, branch)
	},
}

func init() {
	rootCmd.AddCommand(spawnCmd)
}
