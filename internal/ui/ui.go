package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	Purple = lipgloss.Color("99")
	Green  = lipgloss.Color("76")
	Yellow = lipgloss.Color("220")
	Red    = lipgloss.Color("196")
	Gray   = lipgloss.Color("245")
	Cyan   = lipgloss.Color("86")

	// Styles
	Bold    = lipgloss.NewStyle().Bold(true)
	Faint   = lipgloss.NewStyle().Foreground(Gray)
	Success = lipgloss.NewStyle().Foreground(Green).Bold(true)
	Warning = lipgloss.NewStyle().Foreground(Yellow)
	Error   = lipgloss.NewStyle().Foreground(Red).Bold(true)

	// Badges
	BadgeOk   = lipgloss.NewStyle().Background(Green).Foreground(lipgloss.Color("0")).Bold(true).Padding(0, 1)
	BadgeWarn = lipgloss.NewStyle().Background(Yellow).Foreground(lipgloss.Color("0")).Bold(true).Padding(0, 1)
	BadgeErr  = lipgloss.NewStyle().Background(Red).Foreground(lipgloss.Color("0")).Bold(true).Padding(0, 1)
	BadgeInfo = lipgloss.NewStyle().Background(Purple).Foreground(lipgloss.Color("0")).Bold(true).Padding(0, 1)

	// Step prefix
	Step = lipgloss.NewStyle().Foreground(Purple).Bold(true)
)

func StepMsg(icon, msg string) {
	fmt.Printf("%s %s\n", Step.Render(icon), msg)
}

func SuccessMsg(msg string) {
	fmt.Printf("%s %s\n", BadgeOk.Render("OK"), msg)
}

func WarnMsg(msg string) {
	fmt.Printf("%s %s\n", BadgeWarn.Render("WARN"), msg)
}

func ErrorMsg(msg string) {
	fmt.Printf("%s %s\n", BadgeErr.Render("ERR"), msg)
}

// SubMsg prints a dimmed, indented line for subcommand/hook output context
func SubMsg(msg string) {
	fmt.Printf("  %s\n", Faint.Render(msg))
}

// IndentWriter returns a writer that dims and indents each line of output.
// Use it to wrap hook stdout/stderr for a subcommand feel.
func IndentWriter(w io.Writer) io.Writer {
	return &indentWriter{w: w}
}

type indentWriter struct {
	w io.Writer
}

func (iw *indentWriter) Write(p []byte) (int, error) {
	lines := strings.Split(string(p), "\n")
	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			break // skip trailing empty line from split
		}
		fmt.Fprintf(iw.w, "  %s\n", Faint.Render(line))
	}
	return len(p), nil
}
