package docker

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// WorkerInfo represents a hive-managed container discovered via labels.
type WorkerInfo struct {
	ID        string
	Container string
	Status    string
	Created   time.Time
}

// ContainerStats holds resource usage for a running container.
type ContainerStats struct {
	CPU    string
	Memory string
}

// List returns all hive-managed containers, optionally filtered by project.
func List(project string) ([]WorkerInfo, error) {
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

	var workers []WorkerInfo
	for _, name := range lines {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		info, err := inspect(name)
		if err != nil {
			continue
		}
		workers = append(workers, *info)
	}
	return workers, nil
}

// ContainerForWorker returns the container name for a worker by querying labels.
// Returns empty string if no matching container is found.
func ContainerForWorker(project, workerID string) string {
	out, err := exec.Command("docker", "ps", "-a",
		"--filter", "label=hive.worker.id="+workerID,
		"--filter", "label=hive.project="+project,
		"--format", "{{.Names}}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Stats returns CPU and memory usage for a container.
// Returns zero values if the container is not running.
func Stats(name string) ContainerStats {
	out, err := exec.Command("docker", "stats", "--no-stream", "--format", "{{.CPUPerc}}\t{{.MemUsage}}", name).Output()
	if err != nil {
		return ContainerStats{}
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
	if len(parts) != 2 {
		return ContainerStats{}
	}
	mem := parts[1]
	if idx := strings.Index(mem, " / "); idx != -1 {
		mem = mem[:idx]
	}
	return ContainerStats{CPU: parts[0], Memory: mem}
}

type inspectResult struct {
	Name   string        `json:"Name"`
	State  inspectState  `json:"State"`
	Config inspectConfig `json:"Config"`
	Created string       `json:"Created"`
}

type inspectState struct {
	Status string `json:"Status"`
}

type inspectConfig struct {
	Labels map[string]string `json:"Labels"`
}

func inspect(name string) (*WorkerInfo, error) {
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

	return &WorkerInfo{
		ID:        r.Config.Labels["hive.worker.id"],
		Container: strings.TrimPrefix(r.Name, "/"),
		Status:    r.State.Status,
		Created:   created,
	}, nil
}

