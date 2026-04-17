package docker

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nkskaare/hive/internal/config"
)

// CreateAndStart assembles and runs `docker run -d` from the resolved config.
func CreateAndStart(name string, containerCfg *config.ContainerConfig, vars config.Vars) error {
	args := []string{"run", "-d", "--name", name}
	args = append(args, envArgs(containerCfg, vars)...)
	volArgs, err := volumeArgs(containerCfg, vars)
	if err != nil {
		return err
	}
	args = append(args, volArgs...)
	args = append(args, labelArgs(name, containerCfg, vars)...)
	if containerCfg.Workdir != "" {
		args = append(args, "-w", containerCfg.Workdir)
	}
	args = append(args, containerCfg.Image)
	return run("docker", args...)
}

func envArgs(containerCfg *config.ContainerConfig, vars config.Vars) []string {
	var args []string
	for k, v := range containerCfg.Env {
		resolved := config.Resolve(v, vars)
		val := config.ShellEval(resolved)
		if val != "" {
			args = append(args, "-e", k+"="+val)
		}
	}
	return args
}

func volumeArgs(containerCfg *config.ContainerConfig, vars config.Vars) ([]string, error) {
	resolved := make([]string, 0, len(containerCfg.Volumes))
	for _, vol := range containerCfg.Volumes {
		v := config.Resolve(vol, vars)
		parts := strings.SplitN(v, ":", 2)
		if len(parts) == 2 && !filepath.IsAbs(parts[0]) {
			parts[0] = filepath.Join(vars.ConfigDir, parts[0])
			v = strings.Join(parts, ":")
		}
		resolved = append(resolved, v)
	}
	if err := ensureNestedMountPoints(resolved); err != nil {
		return nil, fmt.Errorf("preparing mount points: %w", err)
	}
	var args []string
	for _, v := range resolved {
		args = append(args, "-v", v)
	}
	return args, nil
}

func labelArgs(name string, containerCfg *config.ContainerConfig, vars config.Vars) []string {
	args := []string{
		"-l", "hive=true",
		"-l", "hive.agent.id=" + vars.AgentID,
		"-l", "hive.project=" + vars.Project,
	}
	for k, v := range containerCfg.Labels {
		args = append(args, "-l", k+"="+config.Resolve(v, vars))
	}
	return args
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

// ensureNestedMountPoints creates placeholder files/dirs so Docker Desktop
// (VirtioFS on macOS) can handle bind mounts nested inside other bind mounts.
// For example, if volume A mounts host_dir:/app and volume B mounts
// some_file:/app/.mcp.json, Docker needs .mcp.json to exist inside host_dir.
func ensureNestedMountPoints(volumes []string) error {
	type vol struct {
		host, container string
	}

	var mounts []vol
	for _, v := range volumes {
		parts := strings.SplitN(v, ":", 2)
		if len(parts) != 2 {
			continue
		}
		mounts = append(mounts, vol{host: parts[0], container: parts[1]})
	}

	for i, inner := range mounts {
		for j, outer := range mounts {
			if i == j {
				continue
			}
			// Is inner's container path nested inside outer's container path?
			rel, err := filepath.Rel(outer.container, inner.container)
			if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
				continue
			}
			// inner is nested inside outer — ensure the mount point exists
			// at the corresponding path inside outer's host directory
			placeholder := filepath.Join(outer.host, rel)
			if _, err := os.Stat(placeholder); err == nil {
				continue // already exists
			}

			srcInfo, err := os.Stat(inner.host)
			if err != nil {
				continue // source doesn't exist, Docker will error anyway
			}

			if srcInfo.IsDir() {
				if err := os.MkdirAll(placeholder, 0755); err != nil {
					return fmt.Errorf("creating mount dir %s: %w", placeholder, err)
				}
			} else {
				if err := os.MkdirAll(filepath.Dir(placeholder), 0755); err != nil {
					return fmt.Errorf("creating parent dir for %s: %w", placeholder, err)
				}
				f, err := os.Create(placeholder)
				if err != nil {
					return fmt.Errorf("creating mount placeholder %s: %w", placeholder, err)
				}
				f.Close()
			}
		}
	}
	return nil
}

