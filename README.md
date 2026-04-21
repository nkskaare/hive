# Hive

A thin wrapper around Docker containers and git worktrees. Lets you spin up isolated sandboxes for AI coding agents (or anything else) with one command.

Nothing revolutionary — it just automates the tedious parts: creating a worktree, starting a container with the right mounts and env vars, running setup hooks, and tearing it all down cleanly.

Supports two backends: raw Docker for simple setups, or [devcontainers](https://containers.dev/) when you need features, lifecycle hooks, or a richer dev environment.

## Install

```bash
go install github.com/nkskaare/hive@latest
```

Or build from source:

```bash
git clone https://github.com/nkskaare/hive.git
cd hive
go build -o hive .
```

## Usage

```bash
hive init                          # creates hive.toml + templates
hive spawn ticket-123              # worktree + container + hooks
hive spawn ticket-123 some-branch  # use an existing branch
hive ls                            # show active workers (with CPU/MEM/commits)
hive attach ticket-123             # open/focus terminal tab for a worker
hive run ticket-123 -- bash        # exec into a worker's container
hive run -- bash                   # interactive worker picker, then exec
hive restart ticket-123            # re-create container, keep worktree
hive kill                          # interactive pick + confirm
hive kill ticket-123               # direct teardown
hive kill ticket-123 --keep-branch # keep the git branch
hive nuke                          # tear down all workers at once
```

All commands support `--dry-run` and `--config <path>`.

## What it does

`hive spawn` creates a git worktree, starts a container (via Docker or devcontainer CLI), executes any post-spawn hooks, and optionally opens a terminal tab. `hive kill` reverses all of that. That's it.

## Configuration

`hive init` drops a `hive.toml` in your project root. Template variables use Go template syntax and are resolved at runtime:

```toml
# [project]
# name = "my-project"       # auto-detected from directory name

# [vars]
# User-defined variables — resolved with Go templates, then shell-evaluated.
# Reference built-ins: {{ .Project }}, {{ .ProjectRoot }}, {{ .WorkerID }}, etc.
# Reference other user vars: {{ .Vars.key }}
# git_dir = "{{ .ProjectRoot }}/.git"
# gh_token = "$(gh auth token)"

[worktree]
# root = "../{{ .Project }}.worktrees"
```

### Backend: Docker (default)

Use `[container]` for a straightforward Docker setup. Hive automatically mounts the worktree and `.git` directory into the container, and injects `HIVE_WORKER_ID`, `HIVE_PROJECT`, and `HIVE_PROJECT_ROOT` as env vars.

```toml
[container]
image = "hive-worker:latest"
workdir = "/app"
# Additional volumes (worktree and .git are mounted automatically):
# volumes = ["entrypoint.sh:/usr/local/bin/entrypoint.sh"]
# ports = ["8080:8080", "3000:3000"]

[container.env]
# Shell-evaluated — subshells and $VAR references work
# GH_TOKEN = "$(gh auth token)"

# Build from a Dockerfile instead of pulling an image:
# [container.build]
# dockerfile = "Dockerfile"
# context = "."
```

### Backend: Devcontainer

For more advanced needs — VS Code features, lifecycle hooks, port forwarding rules, etc. — use `[devcontainer]` instead. This delegates to the `devcontainer` CLI and your existing `devcontainer.json`. Hive still auto-mounts `.git` and injects `HIVE_*` env vars.

```toml
[devcontainer]
# config = ".devcontainer/devcontainer.json"  # optional, auto-detected from worktree
```

The two backends are mutually exclusive — include either `[container]` or `[devcontainer]`, not both.

### Hooks, terminal, and shared config

```toml
[hooks]
# post_spawn = ["./scripts/post-spawn.sh {{ .WorkerID }}"]
# pre_kill = ["./scripts/pre-kill.sh {{ .WorkerID }}"]

[terminal]
provider = "none"        # "zellij" | "tmux" | "none"
# layout = "worker-tab.kdl"
# session = "{{ .Project }}"
```

### Built-in variables

| Variable | Example |
|---|---|
| `{{ .Project }}` | `my-project` |
| `{{ .ProjectRoot }}` | `/home/user/my-project` |
| `{{ .WorkerID }}` | `ticket-123` |
| `{{ .Worktree }}` | `/home/user/my-project.worktrees/ticket-123` |
| `{{ .Container }}` | `hive-ticket-123` |
| `{{ .Vars.key }}` | User-defined value from `[vars]` |

The following env vars are also injected into every container automatically:

| Env var | Value |
|---|---|
| `HIVE_WORKER_ID` | The worker ID |
| `HIVE_PROJECT` | The project name |
| `HIVE_PROJECT_ROOT` | Absolute path to the project root |

### User-defined variables

The optional `[vars]` table lets you define your own variables. They're resolved against built-ins first, then shell-evaluated, so both template references and subshells work:

```toml
[vars]
gh_token = "$(gh auth token)"
git_dir = "{{ .ProjectRoot }}/.git"

[container.env]
GH_TOKEN = "{{ .Vars.gh_token }}"

[container]
volumes = [
    "{{ .Vars.git_dir }}:{{ .Vars.git_dir }}",
]
```

## Requirements

- Go 1.24+
- Docker
- Git
- Zellij (optional, for terminal integration)
- devcontainer CLI (optional, `npm install -g @devcontainers/cli`)

## License

Do whatever you want with it.
