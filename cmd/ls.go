package cmd

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/nkskaare/hive/internal/ui"
	"github.com/nkskaare/hive/internal/worker"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List active worker sandboxes",
	RunE: func(cmd *cobra.Command, args []string) error {
		workers, err := worker.List(cfg)
		if err != nil {
			return err
		}

		if len(workers) == 0 {
			fmt.Println(ui.Faint.Render("No active workers."))
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

		for _, w := range workers {
			status := w.Status
			switch status {
			case "running":
				status = lipgloss.NewStyle().Foreground(ui.Green).Render(status)
			case "exited":
				status = lipgloss.NewStyle().Foreground(ui.Red).Render(status)
			default:
				status = lipgloss.NewStyle().Foreground(ui.Yellow).Render(status)
			}
			t.Row(w.ID, w.Branch, w.Container, status, formatDuration(time.Since(w.Created)))
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
