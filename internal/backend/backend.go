package backend

import (
	"github.com/nkskaare/hive/internal/config"
)

// Backend abstracts container lifecycle operations.
// Both Docker and Devcontainer modes implement this interface.
type Backend interface {
	// Up creates and starts the container for a worker.
	// For Docker: builds image (if configured) and runs `docker run`.
	// For Devcontainer: runs `devcontainer up`.
	Up(cfg *config.Config, vars config.Vars) error

	// Down stops and removes the container.
	Down(vars config.Vars) error

	// Exec runs an interactive command inside the container.
	Exec(vars config.Vars, command []string) error

	// ContainerName resolves the actual container name for a worker.
	// Docker returns the deterministic "hive-<id>" name.
	// Devcontainer queries Docker labels since names are auto-generated.
	ContainerName(vars config.Vars) string
}

// New returns the appropriate backend based on config.
func New(cfg *config.Config) Backend {
	if cfg.UseDevcontainer() {
		return &Devcontainer{}
	}
	return &Docker{}
}
