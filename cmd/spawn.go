package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/nkskaare/hive/internal/ui"
	"github.com/nkskaare/hive/internal/worker"
	"github.com/spf13/cobra"
)

var spawnCmd = &cobra.Command{
	Use:   "spawn [id] [branch]",
	Short: "Spawn a new worker sandbox",
	Long:  "Creates a git worktree, starts a Docker container, runs post-spawn hooks, and opens a terminal tab.",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var workerID, branch string

		if len(args) >= 1 {
			workerID = args[0]
		} else {
			err := huh.NewInput().
				Title("Worker ID").
				Placeholder("e.g. fix-login-bug").
				Validate(huh.ValidateNotEmpty()).
				Value(&workerID).
				Run()
			if err != nil {
				return err
			}
		}

		if len(args) >= 2 {
			branch = args[1]
		} else {
			branch = workerID
		}

		if dryRun {
			ui.SubMsg(fmt.Sprintf("Would spawn worker %s on branch %s", ui.Bold.Render(workerID), ui.Bold.Render(branch)))
			return nil
		}

		return worker.Spawn(cfg, workerID, branch, disableHooks)
	},
}

func init() {
	spawnCmd.Flags().BoolVar(&disableHooks, "disable-hooks", false, "skip post-spawn hooks")
	rootCmd.AddCommand(spawnCmd)
}
