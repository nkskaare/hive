package cmd

import (
	"github.com/nkskaare/hive/internal/worker"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <worker-id> -- <command...>",
	Short: "Execute a command in a worker's container",
	Long:  "Runs an interactive command inside the specified worker's Docker container using docker exec.",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		workerID := args[0]
		command := args[1:]
		return worker.Exec(cfg, workerID, command)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
