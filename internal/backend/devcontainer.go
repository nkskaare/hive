package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nkskaare/hive/internal/config"
	"github.com/nkskaare/hive/internal/docker"
)

// Devcontainer implements Backend using the devcontainer CLI.
type Devcontainer struct{}

func (dc *Devcontainer) Up(cfg *config.Config, vars config.Vars) error {
	if _, err := exec.LookPath("devcontainer"); err != nil {
		return fmt.Errorf("devcontainer CLI not found. Install: npm install -g @devcontainers/cli")
	}

	configPath := ""
	if cfg.Devcontainer.Config != "" {
		configPath = cfg.Devcontainer.Config
		if !filepath.IsAbs(configPath) {
			configPath = filepath.Join(cfg.ProjectRoot, configPath)
		}
	}

	args := []string{"up", "--workspace-folder", vars.Worktree}
	if configPath != "" {
		args = append(args, "--config", configPath)
	}
	// Mount .git at its host path so worktree gitdir references resolve
	args = append(args, "--mount",
		fmt.Sprintf("type=bind,source=%s/.git,target=%s/.git", vars.ProjectRoot, vars.ProjectRoot))
	for k, v := range map[string]string{
		"hive":           "true",
		"hive.worker.id": vars.WorkerID,
		"hive.project":   vars.Project,
	} {
		args = append(args, "--id-label", k+"="+v)
	}
	for k, v := range map[string]string{
		"HIVE_WORKER_ID":    vars.WorkerID,
		"HIVE_PROJECT":      vars.Project,
		"HIVE_PROJECT_ROOT": vars.ProjectRoot,
	} {
		args = append(args, "--remote-env", k+"="+v)
	}

	out, err := exec.Command("devcontainer", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("devcontainer up: %s", lastLines(string(out), 5))
	}
	return nil
}

func (dc *Devcontainer) Down(vars config.Vars) error {
	out, err := exec.Command("devcontainer", "down", "--workspace-folder", vars.Worktree).CombinedOutput()
	if err != nil {
		return fmt.Errorf("devcontainer down: %s", lastLines(string(out), 3))
	}
	return nil
}

func (dc *Devcontainer) Exec(vars config.Vars, command []string) error {
	args := []string{"exec", "--workspace-folder", vars.Worktree}
	args = append(args, command...)
	cmd := exec.Command("devcontainer", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (dc *Devcontainer) ContainerName(vars config.Vars) string {
	if name := docker.ContainerForWorker(vars.Project, vars.WorkerID); name != "" {
		return name
	}
	return vars.Container
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= n {
		return strings.TrimSpace(s)
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
