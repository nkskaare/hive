package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/nkskaare/hive/internal/ui"
	"github.com/nkskaare/hive/internal/worker"
	"github.com/spf13/cobra"
)

var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Tear down all worker sandboxes",
	Long:  "Stops all containers, removes all worktrees and branches, and cleans up the terminal session.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		workers, err := worker.List(cfg)
		if err != nil {
			return err
		}
		if len(workers) == 0 {
			fmt.Println("No active workers.")
			return nil
		}

		fmt.Printf("Found %d active worker(s):\n", len(workers))
		for _, w := range workers {
			ui.SubMsg(fmt.Sprintf("%s (%s) [%s]", w.ID, w.Branch, w.Status))
		}
		fmt.Println()

		var confirm bool
		err = huh.NewConfirm().
			Title(fmt.Sprintf("Nuke all %d workers?", len(workers))).
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
		for _, w := range workers {
			if err := worker.Kill(cfg, w.ID, keepBranch, disableHooks); err != nil {
				ui.ErrorMsg(fmt.Sprintf("Failed to kill %s: %v", w.ID, err))
				failed++
			}
		}

		if failed > 0 {
			return fmt.Errorf("%d worker(s) failed to tear down", failed)
		}
		return nil
	},
}

func init() {
	nukeCmd.Flags().BoolVar(&keepBranch, "keep-branch", false, "do not delete git branches after teardown")
	nukeCmd.Flags().BoolVar(&disableHooks, "disable-hooks", false, "skip pre-kill hooks")
	rootCmd.AddCommand(nukeCmd)
}
