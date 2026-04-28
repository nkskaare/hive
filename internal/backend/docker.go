package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nkskaare/hive/internal/config"
)

// Docker implements Backend using raw Docker commands.
type Docker struct{}

func (d *Docker) Up(cfg *config.Config, vars config.Vars) error {
	if cfg.Container.Dockerfile != "" {
		if err := d.build(cfg, vars); err != nil {
			return err
		}
	}
	return d.createAndStart(vars.Container, &cfg.Container, vars)
}

func (d *Docker) Down(vars config.Vars) error {
	dockerRun("stop", vars.Container)
	return dockerRun("rm", "-f", vars.Container)
}

func (d *Docker) Exec(vars config.Vars, command []string) error {
	args := append([]string{"exec", "-it", vars.Container}, command...)
	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (d *Docker) ContainerName(vars config.Vars) string {
	return vars.Container
}

func (d *Docker) build(cfg *config.Config, vars config.Vars) error {
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
	return dockerRun("build", "-t", cfg.Container.Image, "-f", dockerfile, context)
}

func (d *Docker) createAndStart(name string, containerCfg *config.ContainerConfig, vars config.Vars) error {
	args := []string{"run", "-d", "--name", name}

	// Env vars — always inject hive built-ins
	args = append(args,
		"-e", "HIVE_WORKER_ID="+vars.WorkerID,
		"-e", "HIVE_PROJECT="+vars.Project,
		"-e", "HIVE_PROJECT_ROOT="+vars.ProjectRoot,
	)
	for k, v := range containerCfg.Env {
		resolved := config.Resolve(v, vars)
		val := config.ShellEval(resolved)
		if val != "" {
			args = append(args, "-e", k+"="+val)
		}
	}

	// Volumes — always mount worktree and .git (read-only)
	volumes := []string{
		vars.Worktree + ":" + containerCfg.Workdir,
		vars.ProjectRoot + "/.git:" + vars.ProjectRoot + "/.git:ro",
	}
	for _, vol := range containerCfg.Volumes {
		v := config.Resolve(vol, vars)
		parts := strings.SplitN(v, ":", 2)
		if len(parts) == 2 && !filepath.IsAbs(parts[0]) {
			parts[0] = filepath.Join(vars.ProjectRoot, parts[0])
			v = strings.Join(parts, ":")
		}
		volumes = append(volumes, v)
	}
	if err := ensureNestedMountPoints(volumes); err != nil {
		return fmt.Errorf("preparing mount points: %w", err)
	}
	for _, v := range volumes {
		args = append(args, "-v", v)
	}

	// Ports
	for _, p := range containerCfg.Ports {
		args = append(args, "-p", config.Resolve(p, vars))
	}

	// Labels
	args = append(args,
		"-l", "hive=true",
		"-l", "hive.worker.id="+vars.WorkerID,
		"-l", "hive.project="+vars.Project,
	)
	for k, v := range containerCfg.Labels {
		args = append(args, "-l", k+"="+config.Resolve(v, vars))
	}

	if containerCfg.Workdir != "" {
		args = append(args, "-w", containerCfg.Workdir)
	}
	args = append(args, containerCfg.Image)
	return dockerRun(args...)
}

// dockerRun executes a docker CLI command.
func dockerRun(args ...string) error {
	cmd := exec.Command("docker", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker %s: %s", strings.Join(args[:min(3, len(args))], " "), strings.TrimSpace(string(out)))
	}
	return nil
}

// ensureNestedMountPoints creates placeholder files/dirs so Docker Desktop
// (VirtioFS on macOS) can handle bind mounts nested inside other bind mounts.
func ensureNestedMountPoints(volumes []string) error {
	type vol struct{ host, container string }

	var mounts []vol
	for _, v := range volumes {
		parts := strings.SplitN(v, ":", 2)
		if len(parts) == 2 {
			mounts = append(mounts, vol{parts[0], parts[1]})
		}
	}

	for i, inner := range mounts {
		for j, outer := range mounts {
			if i == j {
				continue
			}
			rel, err := filepath.Rel(outer.container, inner.container)
			if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
				continue
			}
			placeholder := filepath.Join(outer.host, rel)
			if _, err := os.Stat(placeholder); err == nil {
				continue
			}
			srcInfo, err := os.Stat(inner.host)
			if err != nil {
				continue
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
