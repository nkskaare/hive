package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hive-sandbox/hive/internal/config"
	"github.com/hive-sandbox/hive/internal/docker"
	"github.com/hive-sandbox/hive/internal/terminal"
	"github.com/hive-sandbox/hive/internal/worktree"
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
	fmt.Printf("📂 Creating worktree for %s on branch %s\n", agentID, branch)
	if err := worktree.Create(cfg.ProjectRoot, vars.Worktree, branch); err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	// 2. Start container
	fmt.Printf("🐳 Starting container %s\n", vars.Container)
	if err := docker.CreateAndStart(vars.Container, &cfg.Container, vars); err != nil {
		worktree.Remove(vars.Worktree)
		return fmt.Errorf("container: %w", err)
	}

	// 3. Run post-spawn hooks
	for _, hook := range cfg.Hooks.PostSpawn {
		resolved := config.Resolve(hook, vars)
		fmt.Printf("🪝 %s\n", resolved)
		if err := runHook(resolved, cfg.ProjectRoot); err != nil {
			fmt.Printf("⚠️  Hook failed: %v\n", err)
		}
	}

	// 4. Start agent CLI inside the container (background)
	if cfg.Agent.Command != "" {
		agentCmd := config.Resolve(cfg.Agent.Command, vars)
		fmt.Printf("🤖 Starting agent: %s\n", agentCmd)
		if err := docker.ExecDetached(vars.Container, agentCmd); err != nil {
			fmt.Printf("⚠️  Agent start: %v\n", err)
		}
	}

	// 5. Terminal tab
	if cfg.Terminal.Layout != "" {
		session := config.Resolve(cfg.Terminal.Session, vars)
		layoutFile, err := resolveLayout(cfg, vars)
		if err != nil {
			fmt.Printf("⚠️  Layout error: %v\n", err)
		} else {
			tp := terminal.NewProvider(cfg.Terminal.Provider)
			if err := tp.AddTab(session, vars.Container, layoutFile); err != nil {
				fmt.Printf("⚠️  Terminal: %v\n", err)
			}
			os.Remove(layoutFile)
		}
	}

	fmt.Printf("✅ Agent %s ready (%s)\n", agentID, vars.Container)
	return nil
}

// Kill tears down an agent: hooks → container → worktree → branch → terminal
func Kill(cfg *config.Config, agentID string, keepBranch bool) error {
	vars := config.AgentVars(cfg, agentID)

	// Infer branch before we destroy the worktree
	branch, _ := worktree.GetBranch(vars.Worktree)

	fmt.Printf("🛑 Killing agent %s\n", agentID)

	// 1. Pre-kill hooks
	for _, hook := range cfg.Hooks.PreKill {
		resolved := config.Resolve(hook, vars)
		fmt.Printf("🪝 %s\n", resolved)
		if err := runHook(resolved, cfg.ProjectRoot); err != nil {
			fmt.Printf("⚠️  Hook failed: %v\n", err)
		}
	}

	// 2. Stop and remove container
	if err := docker.Stop(vars.Container); err != nil {
		fmt.Printf("⚠️  Stop: %v\n", err)
	}
	if err := docker.Remove(vars.Container); err != nil {
		fmt.Printf("⚠️  Remove: %v\n", err)
	}

	// 3. Remove worktree
	if err := worktree.Remove(vars.Worktree); err != nil {
		fmt.Printf("⚠️  Worktree: %v\n", err)
	}

	// 4. Delete branch
	if !keepBranch && branch != "" {
		fmt.Printf("🌿 Deleting branch %s\n", branch)
		if err := worktree.DeleteBranch(cfg.ProjectRoot, branch); err != nil {
			fmt.Printf("⚠️  Branch: %v\n", err)
		}
	}

	// 5. Prune stale worktree refs
	worktree.Prune(cfg.ProjectRoot)

	// 6. Clean up terminal session if no agents remain
	remaining, _ := docker.List(cfg.Project.Name)
	if len(remaining) == 0 {
		tp := terminal.NewProvider(cfg.Terminal.Provider)
		session := config.Resolve(cfg.Terminal.Session, vars)
		tp.RemoveSession(session)
	}

	fmt.Printf("✅ Agent %s cleaned up\n", agentID)
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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
