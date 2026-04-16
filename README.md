# Hive

Agent sandbox orchestrator. Spins up isolated environments for AI coding agents using Docker containers and git worktrees.

Each agent gets its own branch, worktree, and container — fully isolated from other agents and your main working tree.

## Install

```bash
go install github.com/hive-sandbox/hive@latest
```

Or build from source:

```bash
git clone https://github.com/hive-sandbox/hive.git
cd hive
go build -o hive .
```

## Quick start

```bash
# Initialize in your project root
cd my-project
hive init

# Edit hive.toml to point at your Docker image, set env vars, etc.
vim hive.toml

# Spawn an agent — creates worktree + container
hive spawn ticket-123

# Spawn on an existing branch
hive spawn ticket-123 feature/existing-branch

# See what's running
hive ls

# Tear down when done
hive kill ticket-123
```

## How it works

`hive spawn <id>` runs these steps in order:

1. **Git worktree** — creates a worktree on a new branch (or checks out an existing one)
2. **Docker container** — runs `docker run -d` with your configured image, volumes, and env
3. **Post-spawn hooks** — executes any shell commands defined in `[hooks] post_spawn`
4. **Agent command** — starts the agent CLI inside the container in detached mode (if configured)
5. **Terminal tab** — opens a terminal tab via Zellij/tmux (if configured)

`hive kill <id>` reverses the process:

1. **Pre-kill hooks** — runs `[hooks] pre_kill` commands
2. **Container** — stops and removes the container
3. **Worktree** — removes the worktree directory
4. **Branch** — deletes the branch (unless `--keep-branch` is set)
5. **Terminal** — removes the terminal session when no agents remain

## Configuration

`hive init` creates a `hive.toml` in your project root:

```toml
# [project]
# name = "my-project"       # Auto-detected from directory name

[worktree]
# root = "../{project}.worktrees"

[container]
image = "hive-agent:latest"
workdir = "/app"
volumes = [
    "{worktree}:/app",
    "{project_root}/.git:{project_root}/.git",
    "entrypoint.sh:/usr/local/bin/entrypoint.sh",
]

[container.env]
# Env values are shell-evaluated, so subshells and $VAR references work:
# GH_TOKEN = "$(gh auth token)"
# AWS_PROFILE = "$AWS_PROFILE"
TASK_ID = "{agent_id}"
PROJECT_NAME = "{project}"

[agent]
# command = "claude --dangerously-skip-permissions"

[hooks]
# post_spawn = ["./scripts/post-spawn.sh {agent_id}"]
# pre_kill = ["./scripts/pre-kill.sh {agent_id}"]

[terminal]
provider = "none"        # "zellij" | "tmux" | "none"
# layout = "agent-tab.kdl"
# session = "{project}"
```

### Template variables

These placeholders are resolved at runtime in volumes, env values, hooks, agent command, labels, and terminal config:

| Variable | Description |
|---|---|
| `{project}` | Project name (from config or directory name) |
| `{project_root}` | Absolute path to the project root |
| `{agent_id}` | The ID passed to `hive spawn` |
| `{worktree}` | Absolute path to the agent's worktree |
| `{container}` | Container name (`hive-<agent_id>`) |

### Shell-evaluated env values

Env values under `[container.env]` are evaluated as shell expressions before being passed to Docker. This means you can use subshells, environment variables, and fallback chains:

```toml
[container.env]
GH_TOKEN = "$(gh auth token)"
AWS_PROFILE = "$AWS_PROFILE"
API_KEY = "$(cat ~/.secrets/api-key 2>/dev/null || echo '')"
STATIC_VALUE = "hello"
```

Template variables (`{agent_id}`, etc.) are resolved first, then the result is shell-evaluated.

### Agent command

The `[agent]` section configures a CLI command to start inside the container after it's running:

```toml
[agent]
command = "claude --dangerously-skip-permissions"
```

The command runs detached (`docker exec -d`) so it doesn't block the spawn process. Omit or leave empty to skip.

### Hooks

Hooks are shell commands run on the host machine:

```toml
[hooks]
post_spawn = [
    "docker exec {container} npm install",
    "./scripts/setup-agent.sh {agent_id}",
]
pre_kill = [
    "docker exec {container} npm run cleanup",
]
```

### Terminal integration

Hive can open a terminal tab per agent in Zellij (tmux support planned):

```toml
[terminal]
provider = "zellij"
layout = "agent-tab.kdl"
session = "{project}"
```

`hive init` generates a default `agent-tab.kdl` layout with shell, logs, and stats panes. One Zellij session is shared per project, with a tab per agent.

## Commands

### `hive init`

Scaffolds `hive.toml`, `entrypoint.sh`, and `agent-tab.kdl` in the current directory. Skips files that already exist.

### `hive spawn <id> [branch]`

Creates a new agent sandbox. Branch defaults to the agent ID if not specified. If the branch already exists, it checks it out; otherwise it creates a new one from HEAD.

### `hive kill <id>`

Tears down an agent sandbox. Use `--keep-branch` to preserve the git branch after cleanup.

### `hive ls`

Lists all active agent sandboxes for the current project:

```
ID          BRANCH        CONTAINER       STATUS   AGE
ticket-42   ticket-42     hive-ticket-42  running  2h 15m
bugfix-99   fix/login     hive-bugfix-99  running  45m
```

### Global flags

| Flag | Description |
|---|---|
| `--config <path>` | Path to config file (default: walks up to find `hive.toml`) |
| `--dry-run` | Show what would happen without making changes |

## Project structure

```
hive/
├── main.go
├── cmd/
│   ├── root.go          # Cobra root, config loading, global flags
│   ├── init.go          # hive init
│   ├── spawn.go         # hive spawn
│   ├── kill.go          # hive kill
│   └── ls.go            # hive ls
└── internal/
    ├── config/          # TOML parsing, template resolution, shell eval
    ├── agent/           # Spawn/kill/list orchestration
    ├── docker/          # Docker CLI wrapper (run, stop, rm, exec, ls)
    ├── worktree/        # Git worktree operations
    ├── terminal/        # Provider interface (Zellij, noop)
    └── scaffold/        # hive init templates (embedded via go:embed)
```

## Requirements

- Go 1.23+
- Docker
- Git
- Zellij (optional, for terminal integration)

## Design principles

- **Docker passthrough** — hive calls the Docker CLI directly. No SDK, no daemon socket wrapping. Your Docker config, auth, and context work as-is.
- **Agent-agnostic** — hive doesn't know or care what agent CLI you use. Configure it via `[agent] command` or skip it entirely.
- **Minimal** — one config file, four commands. No daemon, no state file, no database.
