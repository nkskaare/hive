package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/nkskaare/hive/internal/worker"
	"github.com/spf13/cobra"
)

var keepBranch bool

var killCmd = &cobra.Command{
	Use:   "kill [id]",
	Short: "Tear down a worker sandbox",
	Long:  "Runs pre-kill hooks, stops the container, removes the worktree and branch, and cleans up the terminal session.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var workerID string

		if len(args) > 0 {
			workerID = args[0]
		} else {
			workers, err := worker.List(cfg)
			if err != nil {
				return err
			}
			if len(workers) == 0 {
				fmt.Println("No active workers.")
				return nil
			}

			options := make([]huh.Option[string], len(workers))
			for i, w := range workers {
				label := fmt.Sprintf("%s (%s) [%s]", w.ID, w.Branch, w.Status)
				options[i] = huh.NewOption(label, w.ID)
			}

			err = huh.NewSelect[string]().
				Title("Select worker to kill").
				Options(options...).
				Value(&workerID).
				Run()
			if err != nil {
				return err
			}
		}

		if dryRun {
			fmt.Printf("Would kill worker %q (keep-branch=%v)\n", workerID, keepBranch)
			return nil
		}

		var confirm bool
		err := huh.NewConfirm().
			Title(fmt.Sprintf("Kill worker %q?", workerID)).
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

		return worker.Kill(cfg, workerID, keepBranch, disableHooks)
	},
}

func init() {
	killCmd.Flags().BoolVar(&keepBranch, "keep-branch", false, "do not delete the git branch after teardown")
	killCmd.Flags().BoolVar(&disableHooks, "disable-hooks", false, "skip pre-kill hooks")
	rootCmd.AddCommand(killCmd)
}
