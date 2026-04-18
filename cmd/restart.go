package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/nkskaare/hive/internal/ui"
	"github.com/nkskaare/hive/internal/worker"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart [id]",
	Short: "Restart a worker's container",
	Long:  "Stops and re-creates the container for an existing worker, keeping the worktree intact.",
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
				fmt.Println(ui.Faint.Render("No active workers."))
				return nil
			}

			options := make([]huh.Option[string], len(workers))
			for i, w := range workers {
				label := fmt.Sprintf("%s (%s) [%s]", w.ID, w.Branch, w.Status)
				options[i] = huh.NewOption(label, w.ID)
			}

			err = huh.NewSelect[string]().
				Title("Select worker to restart").
				Options(options...).
				Value(&workerID).
				Run()
			if err != nil {
				return err
			}
		}

		if dryRun {
			ui.SubMsg(fmt.Sprintf("Would restart worker %s", ui.Bold.Render(workerID)))
			return nil
		}

		return worker.Restart(cfg, workerID, disableHooks)
	},
}

func init() {
	restartCmd.Flags().BoolVar(&disableHooks, "disable-hooks", false, "skip hooks during restart")
	rootCmd.AddCommand(restartCmd)
}
