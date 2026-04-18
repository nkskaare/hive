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
		}).Run(); err != nil {
		return err
	}
	if worktreeErr != nil {
		return fmt.Errorf("worktree: %w", worktreeErr)
	}
	return nil
}

// Start builds (if configured) and starts the container, then runs post-spawn hooks
func Start(cfg *config.Config, workerID string, disableHooks bool) error {
	vars := config.WorkerVars(cfg, workerID)

	// 1. Build image if Dockerfile is configured
	if cfg.Container.Dockerfile != "" {
		if err := buildImage(cfg, vars); err != nil {
			return err
		}
	}

	// 2. Start container
	var containerErr error
	if err := spinner.New().
		Title(fmt.Sprintf("Starting container %s", ui.Bold.Render(vars.Container))).
		Action(func() {
			containerErr = docker.CreateAndStart(vars.Container, &cfg.Container, vars)
		}).Run(); err != nil {
		return err
	}
	if containerErr != nil {
		ui.ErrorMsg(fmt.Sprintf("Container failed: %v", containerErr))
		return fmt.Errorf("container: %w", containerErr)
	}

	// 3. Run post-spawn hooks
	if !disableHooks {
		for _, hook := range cfg.Hooks.PostSpawn {
			resolved := config.Resolve(hook, vars)
			ui.SubMsg(fmt.Sprintf("hook: %s", resolved))
			if err := runHook(resolved, vars); err != nil {
				ui.WarnMsg(fmt.Sprintf("Hook failed: %v", err))
			}
		}
	}

	return nil
}

// Restart stops the container and starts it again, keeping the worktree intact
func Restart(cfg *config.Config, workerID string, disableHooks bool) error {
	vars := config.WorkerVars(cfg, workerID)

	var stopErr error
	if err := spinner.New().
		Title(fmt.Sprintf("Stopping container %s", ui.Bold.Render(vars.Container))).
		Action(func() {
			docker.Stop(vars.Container)
			docker.Remove(vars.Container)
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

	// Infer branch before we destroy the worktree
	branch, _ := worktree.GetBranch(vars.Worktree)

	// 1. Pre-kill hooks
	if !disableHooks {
		for _, hook := range cfg.Hooks.PreKill {
			resolved := config.Resolve(hook, vars)
			ui.SubMsg(fmt.Sprintf("hook: %s", resolved))
			if err := runHook(resolved, vars); err != nil {
				ui.WarnMsg(fmt.Sprintf("Hook failed: %v", err))
			}
		}
	}

	var killErr error
	if err := spinner.New().
		Title(fmt.Sprintf("Stopping worker %s", ui.Bold.Render(workerID))).
		Action(func() {
			docker.Stop(vars.Container)
			docker.Remove(vars.Container)
			worktree.Remove(vars.Worktree)
			if !keepBranch && branch != "" {
				worktree.DeleteBranch(cfg.ProjectRoot, branch)
			}
			worktree.Prune(cfg.ProjectRoot)
		}).Run(); err != nil {
		killErr = err
	}

	// Clean up terminal session if no workers remain
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

// Attach opens or focuses the terminal tab for a worker.
// If the session doesn't have the worker's tab yet, it adds the layout first.
// If no session exists, it creates one.
func Attach(cfg *config.Config, workerID string) error {
	if cfg.Terminal.Layout == "" {
		return nil
	}

	vars := config.WorkerVars(cfg, workerID)
	tp := terminal.NewProvider(cfg.Terminal.Provider)
	session := config.Resolve(cfg.Terminal.Session, vars)

	if !tp.HasSession(session) {
		layoutFile, err := resolveLayout(cfg, vars)
		if err != nil {
			return fmt.Errorf("layout: %w", err)
		}
		defer os.Remove(layoutFile)
		return tp.AddTab(session, vars.Container, layoutFile)
	}

	if !tp.HasTab(session, vars.Container) {
		layoutFile, err := resolveLayout(cfg, vars)
		if err != nil {
			return fmt.Errorf("layout: %w", err)
		}
		defer os.Remove(layoutFile)
		if err := tp.AddTab(session, vars.Container, layoutFile); err != nil {
			return err
		}
	}

	return tp.Attach(session, vars.Container)
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
	args := append([]string{"exec", "-it", vars.Container}, command...)
	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildImage(cfg *config.Config, vars config.Vars) error {
	dockerfile := cfg.Container.Dockerfile
	if !filepath.IsAbs(dockerfile) {
		dockerfile = filepath.Join(cfg.ProjectRoot, dockerfile)
	}
	context := cfg.Container.Context
	if context == "" {
		context = cfg.ProjectRoot
	} else if !filepath.IsAbs(context) {
		context = filepath.Join(cfg.ProjectRoot, context)
	}

	var buildErr error
	if err := spinner.New().
		Title(fmt.Sprintf("Building image %s", ui.Bold.Render(cfg.Container.Image))).
		Action(func() {
			buildErr = docker.Build(cfg.Container.Image, dockerfile, context)
		}).Run(); err != nil {
		return err
	}
	return buildErr
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
