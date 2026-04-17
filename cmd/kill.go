package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/hive-sandbox/hive/internal/agent"
	"github.com/spf13/cobra"
)

var keepBranch bool

var killCmd = &cobra.Command{
	Use:   "kill [id]",
	Short: "Tear down an agent sandbox",
	Long:  "Runs pre-kill hooks, stops the container, removes the worktree and branch, and cleans up the terminal session.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var agentID string

		if len(args) > 0 {
			agentID = args[0]
		} else {
			// Interactive: let user pick from running agents
			agents, err := agent.List(cfg)
			if err != nil {
				return err
			}
			if len(agents) == 0 {
				fmt.Println("No active agents.")
				return nil
			}

			options := make([]huh.Option[string], len(agents))
			for i, a := range agents {
				label := fmt.Sprintf("%s (%s) [%s]", a.ID, a.Branch, a.Status)
				options[i] = huh.NewOption(label, a.ID)
			}

			err = huh.NewSelect[string]().
				Title("Select agent to kill").
				Options(options...).
				Value(&agentID).
				Run()
			if err != nil {
				return err
			}
		}

		if dryRun {
			fmt.Printf("Would kill agent %q (keep-branch=%v)\n", agentID, keepBranch)
			return nil
		}

		// Confirm before teardown
		var confirm bool
		err := huh.NewConfirm().
			Title(fmt.Sprintf("Kill agent %q?", agentID)).
			Description("This will stop the container, remove the worktree, and delete the branch.").
			Affirmative("Yes, kill it").
			Negative("Cancel").
			Value(&confirm).
			Run()
		if err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Cancelled.")
			return nil
		}

		return agent.Kill(cfg, agentID, keepBranch)
	},
}

func init() {
	killCmd.Flags().BoolVar(&keepBranch, "keep-branch", false, "do not delete the git branch after teardown")
	rootCmd.AddCommand(killCmd)
}
