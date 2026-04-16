package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/hive-sandbox/hive/internal/agent"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List active agent sandboxes",
	RunE: func(cmd *cobra.Command, args []string) error {
		agents, err := agent.List(cfg)
		if err != nil {
			return err
		}

		if len(agents) == 0 {
			fmt.Println("No active agents.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tBRANCH\tCONTAINER\tSTATUS\tAGE")
		for _, a := range agents {
			age := formatDuration(time.Since(a.Created))
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", a.ID, a.Branch, a.Container, a.Status, age)
		}
		return w.Flush()
	},
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%dh %dm", hours, int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd %dh", hours/24, hours%24)
}

func init() {
	rootCmd.AddCommand(lsCmd)
}
