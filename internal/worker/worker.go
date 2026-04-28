package worker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/nkskaare/hive/internal/backend"
	"github.com/nkskaare/hive/internal/config"
	"github.com/nkskaare/hive/internal/docker"
	"github.com/nkskaare/hive/internal/terminal"
	"github.com/nkskaare/hive/internal/ui"
	"github.com/nkskaare/hive/internal/worktree"
)

// Worker represents a hive-managed worker sandbox
type Worker struct {
	ID        string
	Branch    string
	Container string
	Worktree  string
	Status    string
	Created   time.Time
	Commits   int    // commits ahead of default branch
	CPU       string // container CPU usage
	Memory    string // container memory usage
}

// Spawn creates a new worker: worktree → container → hooks → terminal
func Spawn(cfg *config.Config, workerID, branch string, disableHooks bool) error {
	vars := config.WorkerVars(cfg, workerID)

	// 1. Handle existing worktree conflict
	if worktree.Exists(vars.Worktree) {
		var choice string
		err := huh.NewSelect[string]().
			Title(fmt.Sprintf("Worktree %s already exists", ui.Bold.Render(workerID))).
			Options(
				huh.NewOption("Overwrite (remove and recreate)", "overwrite"),
				huh.NewOption("Rename (pick a new worker ID)", "rename"),
				huh.NewOption("Cancel", "cancel"),
			).
			Value(&choice).
			Run()
		if err != nil {
			return err
		}
		switch choice {
		case "overwrite":
			ui.SubMsg("removing existing worktree")
			worktree.Remove(vars.Worktree)
			worktree.Prune(cfg.ProjectRoot)
		case "rename":
			var newID string
			if err := huh.NewInput().
				Title("New Worker ID").
				Placeholder(workerID + "-2").
				Validate(huh.ValidateNotEmpty()).
				Value(&newID).
				Run(); err != nil {
				return err
			}
			workerID = newID
			vars = config.WorkerVars(cfg, workerID)
		case "cancel":
			return fmt.Errorf("spawn cancelled")
		}
	}

	if err := Create(cfg, workerID, branch); err != nil {
		return err
	}

	if err := Start(cfg, workerID, disableHooks); err != nil {
		return err
	}

	ui.SuccessMsg(fmt.Sprintf("Worker %s ready (%s)", ui.Bold.Render(workerID), vars.Container))

	if err := Attach(cfg, workerID); err != nil {
		ui.WarnMsg(fmt.Sprintf("Terminal: %v", err))
	}

	return nil
}

// Create sets up the git worktree for a worker
func Create(cfg *config.Config, workerID, branch string) error {
	vars := config.WorkerVars(cfg, workerID)

	var worktreeErr error
	if err := spinner.New().
		Title(fmt.Sprintf("Creating worktree for %s on branch %s", ui.Bold.Render(workerID), ui.Bold.Render(branch))).
		Action(func() {
			worktreeErr = worktree.Create(cfg.ProjectRoot, vars.Worktree, branch)
			if worktreeErr == nil {
				worktreeErr = worktree.IsolateGitDir(cfg.ProjectRoot, vars.Worktree, workerID)
			}
		}).Run(); err != nil {
		return err
	}
	if worktreeErr != nil {
		return fmt.Errorf("worktree: %w", worktreeErr)
	}
	return nil
}

// Start starts the container backend, then runs post-spawn hooks.
// In async mode (default), hooks run in the background with output written
// to <worktree>/.hive/post-spawn.log. In sync mode, hooks run inline.
func Start(cfg *config.Config, workerID string, disableHooks bool) error {
	vars := config.WorkerVars(cfg, workerID)
	be := backend.New(cfg)

	var upErr error
	if err := spinner.New().
		Title(fmt.Sprintf("Starting worker %s", ui.Bold.Render(workerID))).
		Action(func() {
			upErr = be.Up(cfg, vars)
		}).Run(); err != nil {
		return err
	}
	if upErr != nil {
		ui.ErrorMsg(fmt.Sprintf("Container failed: %v", upErr))
		return fmt.Errorf("container: %w", upErr)
	}

	// Resolve container name (may differ from vars.Container in devcontainer mode)
	vars.Container = be.ContainerName(vars)

	if !disableHooks && len(cfg.Hooks.PostSpawn) > 0 {
		if cfg.Hooks.IsAsync() {
			logPath := filepath.Join(vars.Worktree, ".hive", "post-spawn.log")
			go runHooksAsync(cfg.Hooks.PostSpawn, vars, logPath)
			ui.SubMsg(fmt.Sprintf("hooks running in background → %s", logPath))
		} else {
			for _, hook := range cfg.Hooks.PostSpawn {
				resolved := config.Resolve(hook, vars)
				ui.SubMsg(fmt.Sprintf("hook: %s", resolved))
				if err := runHook(resolved, vars); err != nil {
					ui.WarnMsg(fmt.Sprintf("Hook failed: %v", err))
				}
			}
		}
	}

	return nil
}

// Restart stops the container and starts it again, keeping the worktree intact
func Restart(cfg *config.Config, workerID string, disableHooks bool) error {
	vars := config.WorkerVars(cfg, workerID)
	be := backend.New(cfg)

	var stopErr error
	if err := spinner.New().
		Title(fmt.Sprintf("Stopping worker %s", ui.Bold.Render(workerID))).
		Action(func() {
			stopErr = be.Down(vars)
		}).Run(); err != nil {
		return err
	}
	if stopErr != nil {
		return stopErr
	}

	if err := Start(cfg, workerID, disableHooks); err != nil {
		return err
	}
	ui.SuccessMsg(fmt.Sprintf("Worker %s restarted", ui.Bold.Render(workerID)))
	return nil
}

// Kill tears down a worker: hooks → container → worktree → branch → terminal
func Kill(cfg *config.Config, workerID string, keepBranch, disableHooks bool) error {
	vars := config.WorkerVars(cfg, workerID)
	be := backend.New(cfg)
	vars.Container = be.ContainerName(vars)

	branch, _ := worktree.GetBranch(vars.Worktree)

	if !disableHooks && len(cfg.Hooks.PreKill) > 0 {
		if cfg.Hooks.IsAsync() {
			logPath := filepath.Join(vars.Worktree, ".hive", "pre-kill.log")
			var hookErr error
			if err := spinner.New().
				Title("Running pre-kill hooks").
				Action(func() {
					hookErr = runHooksToFile(cfg.Hooks.PreKill, vars, logPath)
				}).Run(); err != nil {
				return err
			}
			if hookErr != nil {
				ui.WarnMsg(fmt.Sprintf("Hook failed (see %s)", logPath))
			}
		} else {
			for _, hook := range cfg.Hooks.PreKill {
				resolved := config.Resolve(hook, vars)
				ui.SubMsg(fmt.Sprintf("hook: %s", resolved))
				if err := runHook(resolved, vars); err != nil {
					ui.WarnMsg(fmt.Sprintf("Hook failed: %v", err))
				}
			}
		}
	}

	var killErr error
	if err := spinner.New().
		Title(fmt.Sprintf("Stopping worker %s", ui.Bold.Render(workerID))).
		Action(func() {
			be.Down(vars)
			worktree.Remove(vars.Worktree)
			worktree.Prune(cfg.ProjectRoot)
			if !keepBranch && branch != "" {
				worktree.DeleteBranch(cfg.ProjectRoot, branch)
			}
		}).Run(); err != nil {
		killErr = err
	}

	remaining, _ := docker.List(cfg.Project.Name)
	if len(remaining) == 0 {
		tp := terminal.NewProvider(cfg.Terminal.Provider)
		session := config.Resolve(cfg.Terminal.Session, vars)
		tp.RemoveSession(session)
	}

	if killErr != nil {
		return killErr
	}

	ui.SuccessMsg(fmt.Sprintf("Worker %s cleaned up", ui.Bold.Render(workerID)))
	return nil
}

// Attach opens or focuses the terminal tab for a worker
func Attach(cfg *config.Config, workerID string) error {
	if cfg.Terminal.Layout == "" {
		return nil
	}

	vars := config.WorkerVars(cfg, workerID)
	be := backend.New(cfg)
	vars.Container = be.ContainerName(vars)

	tp := terminal.NewProvider(cfg.Terminal.Provider)
	session := config.Resolve(cfg.Terminal.Session, vars)

	if !tp.HasSession(session) {
		layoutFile, err := resolveLayout(cfg, vars)
		if err != nil {
			return fmt.Errorf("layout: %w", err)
		}
		defer os.Remove(layoutFile)
		return tp.AddTab(session, workerID, layoutFile)
	}

	if !tp.HasTab(session, workerID) {
		layoutFile, err := resolveLayout(cfg, vars)
		if err != nil {
			return fmt.Errorf("layout: %w", err)
		}
		defer os.Remove(layoutFile)
		if err := tp.AddTab(session, workerID, layoutFile); err != nil {
			return err
		}
	}

	return tp.Attach(session, workerID)
}

// List returns all active workers for the project
func List(cfg *config.Config) ([]Worker, error) {
	infos, err := docker.List(cfg.Project.Name)
	if err != nil {
		return nil, err
	}

	workers := make([]Worker, 0, len(infos))
	for _, info := range infos {
		vars := config.WorkerVars(cfg, info.ID)
		branch, _ := worktree.GetBranch(vars.Worktree)
		commits := worktree.CommitCount(cfg.ProjectRoot, vars.Worktree)
		stats := docker.Stats(info.Container)
		workers = append(workers, Worker{
			ID:        info.ID,
			Branch:    branch,
			Container: info.Container,
			Worktree:  vars.Worktree,
			Status:    info.Status,
			Created:   info.Created,
			Commits:   commits,
			CPU:       stats.CPU,
			Memory:    stats.Memory,
		})
	}
	return workers, nil
}

// Exec runs a command inside a worker's container interactively
func Exec(cfg *config.Config, workerID string, command []string) error {
	vars := config.WorkerVars(cfg, workerID)
	be := backend.New(cfg)
	return be.Exec(vars, command)
}

func runHook(command string, vars config.Vars) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = vars.ProjectRoot
	cmd.Stdout = ui.IndentWriter(os.Stdout)
	cmd.Stderr = ui.IndentWriter(os.Stderr)
	cmd.Env = append(os.Environ(),
		"HIVE_WORKER_ID="+vars.WorkerID,
		"HIVE_CONTAINER="+vars.Container,
		"HIVE_PROJECT="+vars.Project,
		"HIVE_PROJECT_ROOT="+vars.ProjectRoot,
		"HIVE_WORKTREE="+vars.Worktree,
	)
	return cmd.Run()
}

func runHooksAsync(hooks []string, vars config.Vars, logPath string) {
	runHooksToFile(hooks, vars, logPath)
}

func runHooksToFile(hooks []string, vars config.Vars, logPath string) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return err
	}
	f, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer f.Close()

	hookEnv := append(os.Environ(),
		"HIVE_WORKER_ID="+vars.WorkerID,
		"HIVE_CONTAINER="+vars.Container,
		"HIVE_PROJECT="+vars.Project,
		"HIVE_PROJECT_ROOT="+vars.ProjectRoot,
		"HIVE_WORKTREE="+vars.Worktree,
	)

	var lastErr error
	for _, hook := range hooks {
		resolved := config.Resolve(hook, vars)
		fmt.Fprintf(f, "==> %s\n", resolved)
		cmd := exec.Command("sh", "-c", resolved)
		cmd.Dir = vars.ProjectRoot
		cmd.Stdout = f
		cmd.Stderr = f
		cmd.Env = hookEnv
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(f, "FAILED: %v\n", err)
			lastErr = err
		}
	}
	fmt.Fprintf(f, "==> hooks complete\n")
	return lastErr
}

func resolveLayout(cfg *config.Config, vars config.Vars) (string, error) {
	layoutPath := cfg.Terminal.Layout
	if !filepath.IsAbs(layoutPath) {
		layoutPath = filepath.Join(cfg.ProjectRoot, layoutPath)
	}

	data, err := os.ReadFile(layoutPath)
	if err != nil {
		return "", fmt.Errorf("reading layout %s: %w", layoutPath, err)
	}

	resolved := resolveLayoutVars(string(data), vars)

	tmp, err := os.CreateTemp("", "hive-layout-*.kdl")
	if err != nil {
		return "", err
	}
	if _, err := tmp.WriteString(resolved); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	tmp.Close()
	return tmp.Name(), nil
}

func resolveLayoutVars(content string, vars config.Vars) string {
	pairs := []string{
		"{container}", vars.Container,
		"{worker_id}", vars.WorkerID,
		"{project}", vars.Project,
		"{project_root}", vars.ProjectRoot,
		"{worktree}", vars.Worktree,
	}
	for k, v := range vars.Vars {
		pairs = append(pairs, "{vars."+k+"}", v)
	}
	return strings.NewReplacer(pairs...).Replace(content)
}
