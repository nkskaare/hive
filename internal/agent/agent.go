package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/charmbracelet/huh/spinner"
	"github.com/nkskaare/hive/internal/config"
	"github.com/nkskaare/hive/internal/docker"
	"github.com/nkskaare/hive/internal/terminal"
	"github.com/nkskaare/hive/internal/ui"
	"github.com/nkskaare/hive/internal/worktree"
)

// Agent represents a hive-managed agent sandbox
type Agent struct {
	ID        string
	Branch    string
	Container string
	Worktree  string
	Status    string
	Created   time.Time
}

// Spawn creates a new agent: worktree → container → hooks → terminal
func Spawn(cfg *config.Config, agentID, branch string) error {
	vars := config.AgentVars(cfg, agentID)

	// 1. Create worktree
	var worktreeErr error
	if err := spinner.New().
		Title(fmt.Sprintf("Creating worktree for %s on branch %s", ui.Bold.Render(agentID), ui.Bold.Render(branch))).
		Action(func() {
			worktreeErr = worktree.Create(cfg.ProjectRoot, vars.Worktree, branch)
		}).Run(); err != nil {
		return err
	}
	if worktreeErr != nil {
		return fmt.Errorf("worktree: %w", worktreeErr)
	}

	// 2. Start container
	var containerErr error
	if err := spinner.New().
		Title(fmt.Sprintf("Starting container %s", ui.Bold.Render(vars.Container))).
		Action(func() {
			containerErr = docker.CreateAndStart(vars.Container, &cfg.Container, vars)
		}).Run(); err != nil {
		worktree.Remove(vars.Worktree)
		return err
	}
	if containerErr != nil {
		ui.ErrorMsg(fmt.Sprintf("Container failed: %v", containerErr))
		ui.SubMsg("cleaning up worktree")
		worktree.Remove(vars.Worktree)
		worktree.Prune(cfg.ProjectRoot)
		return fmt.Errorf("container: %w", containerErr)
	}

	// 3. Run post-spawn hooks
	for _, hook := range cfg.Hooks.PostSpawn {
		resolved := config.Resolve(hook, vars)
		ui.SubMsg(fmt.Sprintf("hook: %s", resolved))
		if err := runHook(resolved, cfg.ProjectRoot); err != nil {
			ui.WarnMsg(fmt.Sprintf("Hook failed: %v", err))
		}
	}

	// 4. Start agent CLI inside the container (background)
	if cfg.Agent.Command != "" {
		agentCmd := config.Resolve(cfg.Agent.Command, vars)
		ui.SubMsg(fmt.Sprintf("agent: %s", agentCmd))
		if err := docker.ExecDetached(vars.Container, agentCmd); err != nil {
			ui.WarnMsg(fmt.Sprintf("Agent start: %v", err))
		}
	}

	ui.SuccessMsg(fmt.Sprintf("Agent %s ready (%s)", ui.Bold.Render(agentID), vars.Container))

	// 5. Terminal tab (may block if creating a new session)
	if cfg.Terminal.Layout != "" {
		session := config.Resolve(cfg.Terminal.Session, vars)
		layoutFile, err := resolveLayout(cfg, vars)
		if err != nil {
			ui.WarnMsg(fmt.Sprintf("Layout error: %v", err))
		} else {
			tp := terminal.NewProvider(cfg.Terminal.Provider)
			if err := tp.AddTab(session, vars.Container, layoutFile); err != nil {
				ui.WarnMsg(fmt.Sprintf("Terminal: %v", err))
			}
			os.Remove(layoutFile)
		}
	}

	return nil
}

// Kill tears down an agent: hooks → container → worktree → branch → terminal
func Kill(cfg *config.Config, agentID string, keepBranch bool) error {
	vars := config.AgentVars(cfg, agentID)

	// Infer branch before we destroy the worktree
	branch, _ := worktree.GetBranch(vars.Worktree)

	var killErr error
	if err := spinner.New().
		Title(fmt.Sprintf("Stopping agent %s", ui.Bold.Render(agentID))).
		Action(func() {
			// 1. Pre-kill hooks
			for _, hook := range cfg.Hooks.PreKill {
				resolved := config.Resolve(hook, vars)
				if err := runHook(resolved, cfg.ProjectRoot); err != nil {
					fmt.Fprintf(os.Stderr, "hook failed: %v\n", err)
				}
			}

			// 2. Stop and remove container
			docker.Stop(vars.Container)
			docker.Remove(vars.Container)

			// 3. Remove worktree
			worktree.Remove(vars.Worktree)

			// 4. Delete branch
			if !keepBranch && branch != "" {
				worktree.DeleteBranch(cfg.ProjectRoot, branch)
			}

			// 5. Prune stale worktree refs
			worktree.Prune(cfg.ProjectRoot)
		}).Run(); err != nil {
		killErr = err
	}

	// 6. Clean up terminal session if no agents remain
	remaining, _ := docker.List(cfg.Project.Name)
	if len(remaining) == 0 {
		tp := terminal.NewProvider(cfg.Terminal.Provider)
		session := config.Resolve(cfg.Terminal.Session, vars)
		tp.RemoveSession(session)
	}

	if killErr != nil {
		return killErr
	}

	ui.SuccessMsg(fmt.Sprintf("Agent %s cleaned up", ui.Bold.Render(agentID)))
	return nil
}

// List returns all active agents for the project
func List(cfg *config.Config) ([]Agent, error) {
	infos, err := docker.List(cfg.Project.Name)
	if err != nil {
		return nil, err
	}

	agents := make([]Agent, 0, len(infos))
	for _, info := range infos {
		vars := config.AgentVars(cfg, info.ID)
		branch, _ := worktree.GetBranch(vars.Worktree)
		agents = append(agents, Agent{
			ID:        info.ID,
			Branch:    branch,
			Container: info.Container,
			Worktree:  vars.Worktree,
			Status:    info.Status,
			Created:   info.Created,
		})
	}
	return agents, nil
}

func runHook(command, dir string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = ui.IndentWriter(os.Stdout)
	cmd.Stderr = ui.IndentWriter(os.Stderr)
	return cmd.Run()
}

func resolveLayout(cfg *config.Config, vars config.Vars) (string, error) {
	layoutPath := cfg.Terminal.Layout
	if !filepath.IsAbs(layoutPath) {
		layoutPath = filepath.Join(cfg.ConfigDir, layoutPath)
	}

	data, err := os.ReadFile(layoutPath)
	if err != nil {
		return "", fmt.Errorf("reading layout %s: %w", layoutPath, err)
	}

	resolved := config.Resolve(string(data), vars)

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
