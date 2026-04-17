package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Zellij implements the Provider interface for the Zellij terminal multiplexer
type Zellij struct{}

func (z *Zellij) Name() string { return "zellij" }

func (z *Zellij) HasSession(session string) bool {
	_, active := z.sessionStatus(session)
	return active
}

// sessionStatus returns whether a session exists and whether it's alive (not exited/dead).
func (z *Zellij) sessionStatus(session string) (exists, alive bool) {
	out, err := exec.Command("zellij", "list-sessions", "--no-formatting").Output()
	if err != nil {
		return false, false
	}
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, session) {
			if strings.Contains(trimmed, "EXITED") {
				return true, false
			}
			return true, true
		}
	}
	return false, false
}

// AddTab creates a session or adds a tab to an existing one using the layout file
func (z *Zellij) AddTab(session, containerName, layoutFile string) error {
	exists, alive := z.sessionStatus(session)
	if exists && alive {
		// Session is running: add tab by targeting the session via env var.
		cmd := exec.Command("zellij", "--session", session, "action", "new-tab", "--name", containerName, "--layout", layoutFile)
		return cmd.Run()
	}
	if exists && !alive {
		// Session exited: delete it and create fresh
		z.RemoveSession(session)
	}
	// No session (or cleaned up exited one): create a new one with the layout
	cmd := exec.Command("zellij", "--session", session, "--new-session-with-layout", layoutFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (z *Zellij) RemoveSession(session string) error {
	return exec.Command("zellij", "delete-session", session).Run()
}

// HasTab checks if a tab with the given name exists in the session
func (z *Zellij) HasTab(session, tabName string) bool {
	cmd := exec.Command("zellij", "--session", session, "action", "query-tab-names")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == tabName {
			return true
		}
	}
	return false
}

// Attach connects to an existing zellij session and focuses the named tab
func (z *Zellij) Attach(session, tabName string) error {
	exists, alive := z.sessionStatus(session)
	if exists && !alive {
		// Dead session: delete and let caller recreate
		z.RemoveSession(session)
		return fmt.Errorf("session %q was dead and has been cleaned up; re-run to create a new one", session)
	}
	// Focus the worker's tab before attaching so the user lands on it
	if tabName != "" {
		_ = exec.Command("zellij", "--session", session, "action", "go-to-tab-name", tabName).Run()
	}
	cmd := exec.Command("zellij", "attach", session)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
