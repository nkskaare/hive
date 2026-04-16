package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Config represents the full hive.toml configuration
type Config struct {
	Project   ProjectConfig   `toml:"project"`
	Worktree  WorktreeConfig  `toml:"worktree"`
	Container ContainerConfig `toml:"container"`
	Agent     AgentConfig     `toml:"agent"`
	Hooks     HooksConfig     `toml:"hooks"`
	Terminal  TerminalConfig  `toml:"terminal"`

	// Resolved at load time, not from TOML
	ProjectRoot string `toml:"-"`
	ConfigDir   string `toml:"-"`
}

type ProjectConfig struct {
	Name string `toml:"name"`
}

type WorktreeConfig struct {
	Root string `toml:"root"`
}

// ContainerConfig maps directly to Docker container concepts.
// Volumes use Docker-native "host:container" syntax.
// Env values are evaluated as shell expressions via `sh -c`.
type ContainerConfig struct {
	Image   string            `toml:"image"`
	Workdir string            `toml:"workdir"`
	Env     map[string]string `toml:"env"`
	Volumes []string          `toml:"volumes"`
	Labels  map[string]string `toml:"labels"`
}

// AgentConfig configures the AI agent CLI to run inside the container
type AgentConfig struct {
	Command string `toml:"command"`
}

type HooksConfig struct {
	PostSpawn []string `toml:"post_spawn"`
	PreKill   []string `toml:"pre_kill"`
}

type TerminalConfig struct {
	Provider string `toml:"provider"`
	Layout   string `toml:"layout"`
	Session  string `toml:"session"`
}

// Vars holds template variables resolved at runtime
type Vars struct {
	Project     string
	ProjectRoot string
	AgentID     string
	Worktree    string
	Container   string
}

// Load reads and parses a hive.toml config file.
// If path is empty, walks up from cwd to find one.
func Load(path string) (*Config, error) {
	if path == "" {
		var err error
		path, err = findConfigFile()
		if err != nil {
			return nil, err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	cfg.ConfigDir = filepath.Dir(absPath)
	cfg.ProjectRoot = cfg.ConfigDir

	if cfg.Project.Name == "" {
		cfg.Project.Name = filepath.Base(cfg.ProjectRoot)
	}
	if cfg.Worktree.Root == "" {
		cfg.Worktree.Root = "../{project}.worktrees"
	}
	if cfg.Container.Workdir == "" {
		cfg.Container.Workdir = "/app"
	}
	if cfg.Terminal.Provider == "" {
		cfg.Terminal.Provider = "none"
	}
	if cfg.Terminal.Session == "" {
		cfg.Terminal.Session = "{project}"
	}

	return &cfg, nil
}

// AgentVars returns the full set of variables for a specific agent
func AgentVars(cfg *Config, agentID string) Vars {
	v := Vars{
		Project:     cfg.Project.Name,
		ProjectRoot: cfg.ProjectRoot,
		AgentID:     agentID,
		Container:   "hive-" + agentID,
	}

	worktreeRoot := Resolve(cfg.Worktree.Root, v)
	if !filepath.IsAbs(worktreeRoot) {
		worktreeRoot = filepath.Join(cfg.ProjectRoot, worktreeRoot)
	}
	v.Worktree = filepath.Join(worktreeRoot, agentID)

	return v
}

// Resolve replaces {template} variables in a string
func Resolve(s string, v Vars) string {
	r := strings.NewReplacer(
		"{project}", v.Project,
		"{project_root}", v.ProjectRoot,
		"{agent_id}", v.AgentID,
		"{worktree}", v.Worktree,
		"{container}", v.Container,
	)
	return r.Replace(s)
}

// ShellEval evaluates a string as a shell expression.
// Supports $(...) subshells, $VAR env vars, and || fallback chains.
func ShellEval(expr string) string {
	out, err := exec.Command("sh", "-c", fmt.Sprintf("echo %s", expr)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func findConfigFile() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		path := filepath.Join(dir, "hive.toml")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("hive.toml not found. Run 'hive init' to create one")
}
