package cmd

import (
	"fmt"

	"github.com/hive-sandbox/hive/internal/agent"
	"github.com/spf13/cobra"
)

var spawnCmd = &cobra.Command{
	Use:   "spawn <id> [branch]",
	Short: "Spawn a new agent sandbox",
	Long:  "Creates a git worktree, starts a Docker container, runs post-spawn hooks, and opens a terminal tab.",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID := args[0]
		branch := agentID
		if len(args) > 1 {
			branch = args[1]
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
