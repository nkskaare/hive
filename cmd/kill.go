package cmd

import (
	"fmt"

	"github.com/hive-sandbox/hive/internal/agent"
	"github.com/spf13/cobra"
)

var keepBranch bool

var killCmd = &cobra.Command{
	Use:   "kill <id>",
	Short: "Tear down an agent sandbox",
	Long:  "Runs pre-kill hooks, stops the container, removes the worktree and branch, and cleans up the terminal session.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID := args[0]

		if dryRun {
			fmt.Printf("Would kill agent %q (keep-branch=%v)\n", agentID, keepBranch)
			return nil
		}

		return agent.Kill(cfg, agentID, keepBranch)
	},
}

func init() {
	killCmd.Flags().BoolVar(&keepBranch, "keep-branch", false, "do not delete the git branch after teardown")
	rootCmd.AddCommand(killCmd)
}
