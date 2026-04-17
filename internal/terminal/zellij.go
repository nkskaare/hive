package terminal

import (
	"os"
	"os/exec"
	"strings"
)

// Zellij implements the Provider interface for the Zellij terminal multiplexer
type Zellij struct{}

func (z *Zellij) Name() string { return "zellij" }

func (z *Zellij) HasSession(session string) bool {
	out, err := exec.Command("zellij", "list-sessions").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), session) {
			return true
		}
	}
	return false
}

// AddTab creates a session or adds a tab to an existing one using the layout file
func (z *Zellij) AddTab(session, containerName, layoutFile string) error {
	if z.HasSession(session) {
		// Add a new tab to the running session (non-blocking)
		return exec.Command("zellij", "--session", session, "action", "new-tab", "--layout", layoutFile).Run()
	}
	// Create a new session with the layout (attaches interactively)
	cmd := exec.Command("zellij", "--session", session, "--layout", layoutFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (z *Zellij) RemoveSession(session string) error {
	return exec.Command("zellij", "delete-session", session, "--force").Run()
}
