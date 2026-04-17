package docker

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nkskaare/hive/internal/config"
)

// CreateAndStart assembles and runs `docker run -d` from the resolved config.
// Volume paths, env vars, and labels are passed through directly.
func CreateAndStart(name string, containerCfg *config.ContainerConfig, vars config.Vars) error {
	args := []string{"run", "-d", "--name", name}

	for k, v := range containerCfg.Env {
		// Resolve {template} vars first, then shell-evaluate the result
		resolved := config.Resolve(v, vars)
		val := config.ShellEval(resolved)
		if val != "" {
			args = append(args, "-e", k+"="+val)
		}
	}

	for _, vol := range containerCfg.Volumes {
		resolved := config.Resolve(vol, vars)
		// Make the host side absolute if it isn't already
		parts := strings.SplitN(resolved, ":", 2)
		if len(parts) == 2 && !filepath.IsAbs(parts[0]) {
			parts[0] = filepath.Join(vars.ProjectRoot, parts[0])
			resolved = strings.Join(parts, ":")
		}
		args = append(args, "-v", resolved)
	}

	// Hive labels for identification
	args = append(args, "-l", "hive=true")
	args = append(args, "-l", "hive.agent.id="+vars.AgentID)
	args = append(args, "-l", "hive.project="+vars.Project)
	for k, v := range containerCfg.Labels {
		args = append(args, "-l", k+"="+config.Resolve(v, vars))
	}

	if containerCfg.Workdir != "" {
		args = append(args, "-w", containerCfg.Workdir)
	}

	args = append(args, containerCfg.Image)

	return run("docker", args...)
}

// ExecDetached runs a command inside a container in detached mode
func ExecDetached(container, command string) error {
	return run("docker", "exec", "-d", container, "sh", "-c", command)
}

// Stop stops a container gracefully
func Stop(name string) error {
	return run("docker", "stop", name)
}

// Remove forcefully removes a container
func Remove(name string) error {
	return run("docker", "rm", "-f", name)
}

// AgentInfo represents a running hive-managed agent container
type AgentInfo struct {
	ID        string
	Container string
	Status    string
	Created   time.Time
}

type inspectResult struct {
	Name    string            `json:"Name"`
	State   inspectState      `json:"State"`
	Config  inspectConfig     `json:"Config"`
	Created string            `json:"Created"`
}

type inspectState struct {
	Status string `json:"Status"`
}

type inspectConfig struct {
	Labels map[string]string `json:"Labels"`
}

// List returns all hive-managed containers, optionally filtered by project
func List(project string) ([]AgentInfo, error) {
	args := []string{"ps", "-a", "--filter", "label=hive=true", "--format", "{{.Names}}"}
	if project != "" {
		args = append(args, "--filter", "label=hive.project="+project)
	}

	out, err := exec.Command("docker", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}

	var agents []AgentInfo
	for _, name := range lines {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		info, err := inspect(name)
		if err != nil {
			continue
		}
		agents = append(agents, *info)
	}
	return agents, nil
}

func inspect(name string) (*AgentInfo, error) {
	out, err := exec.Command("docker", "inspect", name).Output()
	if err != nil {
		return nil, err
	}

	var results []inspectResult
	if err := json.Unmarshal(out, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("container %s not found", name)
	}

	r := results[0]
	created, _ := time.Parse(time.RFC3339Nano, r.Created)

	return &AgentInfo{
		ID:        r.Config.Labels["hive.agent.id"],
		Container: strings.TrimPrefix(r.Name, "/"),
		Status:    r.State.Status,
		Created:   created,
	}, nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %s", name, strings.Join(args[:min(3, len(args))], " "), strings.TrimSpace(string(out)))
	}
	return nil
}

