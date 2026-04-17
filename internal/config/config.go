package config

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pelletier/go-toml/v2"
)

// Config represents the full hive.toml configuration
type Config struct {
	Project   ProjectConfig   `toml:"project"`
	Worktree  WorktreeConfig  `toml:"worktree"`
	Container ContainerConfig `toml:"container"`
	Hooks     HooksConfig     `toml:"hooks"`
	Terminal  TerminalConfig    `toml:"terminal"`
	UserVars  map[string]string `toml:"vars"`

	// Resolved at load time, not from TOML
	ProjectRoot string `toml:"-"`
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
	Image      string            `toml:"image"`
	Dockerfile string            `toml:"dockerfile"`
	Context    string            `toml:"context"`
	Workdir    string            `toml:"workdir"`
	Env        map[string]string `toml:"env"`
	Volumes    []string          `toml:"volumes"`
	Ports      []string          `toml:"ports"`
	Labels     map[string]string `toml:"labels"`
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

// Vars holds template variables resolved at runtime.
// Built-in fields are available as {{ .Project }}, {{ .WorkerID }}, etc.
// User-defined vars from [vars] are available as {{ .Vars.key }}.
type Vars struct {
	Project     string
	ProjectRoot string
	WorkerID    string
	Worktree    string
	Container   string
	Vars        map[string]string
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
	cfg.ProjectRoot = resolveProjectRoot(filepath.Dir(absPath))

	if cfg.Project.Name == "" {
		cfg.Project.Name = filepath.Base(cfg.ProjectRoot)
	}
	if cfg.Worktree.Root == "" {
		cfg.Worktree.Root = "../{{ .Project }}.worktrees"
	}
	if cfg.Container.Workdir == "" {
		cfg.Container.Workdir = "/app"
	}
	if cfg.Terminal.Provider == "" {
		cfg.Terminal.Provider = "none"
	}
	if cfg.Terminal.Session == "" {
		cfg.Terminal.Session = "{{ .Project }}"
	}

	return &cfg, nil
}

// WorkerVars returns the full set of variables for a specific worker
func WorkerVars(cfg *Config, workerID string) Vars {
	v := Vars{
		Project:     cfg.Project.Name,
		ProjectRoot: cfg.ProjectRoot,
		WorkerID:    workerID,
		Container:   "hive-" + workerID,
	}

	worktreeRoot := Resolve(cfg.Worktree.Root, v)
	if !filepath.IsAbs(worktreeRoot) {
		worktreeRoot = filepath.Join(cfg.ProjectRoot, worktreeRoot)
	}
	v.Worktree = filepath.Join(worktreeRoot, workerID)

	// Resolve user-defined vars: template expansion first, then shell eval
	if len(cfg.UserVars) > 0 {
		v.Vars = make(map[string]string, len(cfg.UserVars))
		for k, val := range cfg.UserVars {
			resolved := Resolve(val, v)
			v.Vars[k] = ShellEval(resolved)
		}
	}

	return v
}

// Resolve replaces {{ .Var }} template variables in a string.
// Returns the original string if the template is invalid.
func Resolve(s string, v Vars) string {
	tmpl, err := template.New("").Option("missingkey=zero").Parse(s)
	if err != nil {
		return s
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, v); err != nil {
		return s
	}
	return buf.String()
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
		candidate := filepath.Join(dir, "hive.toml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("hive.toml not found in project root. Run 'hive init' or use --config")
}

// resolveProjectRoot walks up from dir to find the git repository root.
// Falls back to dir itself if no .git is found.
func resolveProjectRoot(dir string) string {
	d := dir
	for {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return dir
}
