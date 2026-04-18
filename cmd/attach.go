package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/nkskaare/hive/internal/ui"
	"github.com/nkskaare/hive/internal/worker"
	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach [worker-id]",
	Short: "Attach to a worker's terminal tab",
	Long:  "Connects to the project's terminal session and focuses the specified worker's tab. Adds the layout tab if it doesn't exist yet.",
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
				Title("Select worker to attach to").
				Options(options...).
				Value(&workerID).
				Run()
			if err != nil {
				return err
			}
		}

		return worker.Attach(cfg, workerID)
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)
}
