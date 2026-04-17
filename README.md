# Hive

A thin wrapper around Docker containers and git worktrees. Lets you spin up isolated sandboxes for AI coding agents (or anything else) with one command.

Nothing revolutionary — it just automates the tedious parts: creating a worktree, starting a container with the right mounts and env vars, running setup hooks, and tearing it all down cleanly.

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
hive ls                            # show running agents
hive kill                          # interactive pick + confirm
hive kill ticket-123               # direct teardown
hive kill ticket-123 --keep-branch # keep the git branch
```

All commands support `--dry-run` and `--config <path>`.

## What it does

`hive spawn` creates a git worktree, runs `docker run` with your config, executes any post-spawn hooks, and optionally opens a terminal tab. `hive kill` reverses all of that. That's it.

## Configuration

`hive init` drops a `hive.toml` in your project root. Template variables use Go template syntax and are resolved at runtime:

```toml
# [project]
# name = "my-project"       # auto-detected from directory name

# [vars]
# User-defined variables — resolved with Go templates, then shell-evaluated.
# Reference built-ins: {{ .Project }}, {{ .ProjectRoot }}, {{ .AgentID }}, etc.
# Reference other user vars: {{ .Vars.key }}
# git_dir = "{{ .ProjectRoot }}/.git"
# gh_token = "$(gh auth token)"

[worktree]
# root = "../{{ .Project }}.worktrees"

[container]
image = "hive-agent:latest"
workdir = "/app"
volumes = [
    "{{ .Worktree }}:/app",
    "{{ .ProjectRoot }}/.git:{{ .ProjectRoot }}/.git",
    "entrypoint.sh:/usr/local/bin/entrypoint.sh",
]

[container.env]
# Shell-evaluated — subshells and $VAR references work
# GH_TOKEN = "$(gh auth token)"
TASK_ID = "{{ .AgentID }}"
PROJECT_NAME = "{{ .Project }}"

[agent]
# command = "claude --dangerously-skip-permissions"

[hooks]
# post_spawn = ["./scripts/post-spawn.sh {{ .AgentID }}"]
# pre_kill = ["./scripts/pre-kill.sh {{ .AgentID }}"]

[terminal]
provider = "none"        # "zellij" | "tmux" | "none"
# layout = "agent-tab.kdl"
# session = "{{ .Project }}"
```

### Built-in variables

| Variable | Example |
|---|---|
| `{{ .Project }}` | `my-project` |
| `{{ .ProjectRoot }}` | `/home/user/my-project` |
| `{{ .AgentID }}` | `ticket-123` |
| `{{ .Worktree }}` | `/home/user/my-project.worktrees/ticket-123` |
| `{{ .Container }}` | `hive-ticket-123` |
| `{{ .Vars.key }}` | User-defined value from `[vars]` |

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

## License

Do whatever you want with it.
