package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/nkskaare/hive/internal/agent"
	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach [agent-id]",
	Short: "Attach to an agent's terminal tab",
	Long:  "Connects to the project's terminal session and focuses the specified agent's tab. Adds the layout tab if it doesn't exist yet.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var agentID string
		if len(args) > 0 {
			agentID = args[0]
		} else {
			agents, err := agent.List(cfg)
			if err != nil {
				return err
			}
			if len(agents) == 0 {
				fmt.Println("No active agents.")
				return nil
			}
			options := make([]huh.Option[string], len(agents))
			for i, a := range agents {
				label := fmt.Sprintf("%s (%s) [%s]", a.ID, a.Branch, a.Status)
				options[i] = huh.NewOption(label, a.ID)
			}
			err = huh.NewSelect[string]().
				Title("Select agent to attach to").
				Options(options...).
				Value(&agentID).
				Run()
			if err != nil {
				return err
			}
		}

		return agent.Attach(cfg, agentID)
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)
}
