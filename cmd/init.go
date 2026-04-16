package cmd

import (
	"fmt"

	"github.com/hive-sandbox/hive/internal/scaffold"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize hive in the current directory",
	Long:  "Creates a hive.toml config file and default templates in the current directory.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := scaffold.Init("."); err != nil {
			return err
		}
		fmt.Println("✅ Hive initialized. Edit hive.toml to configure your project.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
