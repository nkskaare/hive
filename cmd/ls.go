package cmd

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/nkskaare/hive/internal/agent"
	"github.com/nkskaare/hive/internal/ui"
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
			fmt.Println(ui.Faint.Render("No active agents."))
			return nil
		}

		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(ui.Purple)).
			Headers("ID", "BRANCH", "CONTAINER", "STATUS", "AGE").
			StyleFunc(func(row, col int) lipgloss.Style {
				s := lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
				if row == table.HeaderRow {
					return s.Bold(true).Foreground(ui.Purple)
				}
				if row%2 == 0 {
					return s.Foreground(ui.Gray)
				}
				return s
			})

		for _, a := range agents {
			status := a.Status
			switch status {
			case "running":
				status = lipgloss.NewStyle().Foreground(ui.Green).Render(status)
			case "exited":
				status = lipgloss.NewStyle().Foreground(ui.Red).Render(status)
			default:
				status = lipgloss.NewStyle().Foreground(ui.Yellow).Render(status)
			}
			t.Row(a.ID, a.Branch, a.Container, status, formatDuration(time.Since(a.Created)))
		}

		fmt.Println(t)
		return nil
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
