package terminal

import "fmt"

// Provider is the interface for terminal multiplexer integrations.
// Implementations manage sessions and tabs for worker monitoring.
type Provider interface {
	// AddTab adds a tab for a worker to the session.
	// Creates the session if it doesn't exist.
	AddTab(session, containerName, layoutFile string) error

	// Attach connects to an existing session and focuses the named tab
	Attach(session, tabName string) error

	// HasSession checks if a named session exists
	HasSession(session string) bool

	// HasTab checks if a named tab exists in the session
	HasTab(session, tabName string) bool

	// RemoveSession removes a session
	RemoveSession(session string) error

	// Name returns the provider name
	Name() string
}

// NewProvider creates a terminal provider by name
func NewProvider(name string) Provider {
	switch name {
	case "zellij":
		return &Zellij{}
	default:
		return &Noop{}
	}
}

// Noop is a no-op provider for when no terminal multiplexer is configured
type Noop struct{}

func (n *Noop) AddTab(_, _, _ string) error    { return nil }
func (n *Noop) Attach(_, _ string) error        { return fmt.Errorf("no terminal provider configured") }
func (n *Noop) HasSession(_ string) bool        { return false }
func (n *Noop) HasTab(_, _ string) bool          { return false }
func (n *Noop) RemoveSession(_ string) error    { return nil }
func (n *Noop) Name() string                    { return "none" }
