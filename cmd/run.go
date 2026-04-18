package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/nkskaare/hive/internal/ui"
	"github.com/nkskaare/hive/internal/worker"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [worker-id] -- <command...>",
	Short: "Execute a command in a worker's container",
	Long:  "Runs an interactive command inside the specified worker's Docker container using docker exec.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// If first arg looks like a command (no worker selected), prompt
		var workerID string
		var command []string

		workers, err := worker.List(cfg)
		if err != nil {
			return err
		}

		// Check if first arg matches a known worker ID
		found := false
		for _, w := range workers {
			if w.ID == args[0] {
				found = true
				break
			}
		}

		if found {
			workerID = args[0]
			command = args[1:]
		} else {
			// First arg is part of the command; prompt for worker
			command = args
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
				Title("Select worker").
				Options(options...).
				Value(&workerID).
				Run()
			if err != nil {
				return err
			}
		}

		if len(command) == 0 {
			command = []string{"bash", "-l"}
		}

		return worker.Exec(cfg, workerID, command)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
