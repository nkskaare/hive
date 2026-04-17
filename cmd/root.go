package cmd

import (
	"fmt"
	"os"

	"github.com/nkskaare/hive/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile      string
	dryRun       bool
	disableHooks bool
	cfg          *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "hive",
	Short: "Worker sandbox orchestrator",
	Long:  "Hive orchestrates isolated worker sandboxes using Docker containers and git worktrees.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for init and help
		if cmd.Name() == "init" || cmd.Name() == "help" {
			return nil
		}
		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		return nil
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: hive.toml in project root)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would happen without making changes")
}
