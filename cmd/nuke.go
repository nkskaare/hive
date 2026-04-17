package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/nkskaare/hive/internal/agent"
	"github.com/nkskaare/hive/internal/ui"
	"github.com/spf13/cobra"
)

var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Tear down all agent sandboxes",
	Long:  "Stops all containers, removes all worktrees and branches, and cleans up the terminal session.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		agents, err := agent.List(cfg)
		if err != nil {
			return err
		}
		if len(agents) == 0 {
			fmt.Println("No active agents.")
			return nil
		}

		fmt.Printf("Found %d active agent(s):\n", len(agents))
		for _, a := range agents {
			ui.SubMsg(fmt.Sprintf("%s (%s) [%s]", a.ID, a.Branch, a.Status))
		}
		fmt.Println()

		var confirm bool
		err = huh.NewConfirm().
			Title(fmt.Sprintf("Nuke all %d agents?", len(agents))).
			Description("This will stop all containers, remove all worktrees, and delete all branches.").
			Affirmative("Yes, nuke everything").
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

		var failed int
		for _, a := range agents {
			if err := agent.Kill(cfg, a.ID, keepBranch, disableHooks); err != nil {
				ui.ErrorMsg(fmt.Sprintf("Failed to kill %s: %v", a.ID, err))
				failed++
			}
		}

		if failed > 0 {
			return fmt.Errorf("%d agent(s) failed to tear down", failed)
		}
		return nil
	},
}

func init() {
	nukeCmd.Flags().BoolVar(&keepBranch, "keep-branch", false, "do not delete git branches after teardown")
	nukeCmd.Flags().BoolVar(&disableHooks, "disable-hooks", false, "skip pre-kill hooks")
	rootCmd.AddCommand(nukeCmd)
}
