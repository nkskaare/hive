package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Create creates a git worktree at worktreePath.
// Creates the branch if it doesn't exist, otherwise checks it out.
func Create(projectRoot, worktreePath, branch string) error {
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return fmt.Errorf("creating worktree parent dir: %w", err)
	}

	if branchExists(projectRoot, branch) {
		return run("git", "-C", projectRoot, "worktree", "add", worktreePath, branch)
	}
	return run("git", "-C", projectRoot, "worktree", "add", worktreePath, "-b", branch)
}

// Remove forcefully removes a git worktree
func Remove(worktreePath string) error {
	return run("git", "worktree", "remove", "--force", worktreePath)
}

// GetBranch returns the branch name checked out in a worktree
func GetBranch(worktreePath string) (string, error) {
	out, err := exec.Command("git", "-C", worktreePath, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("getting branch for worktree %s: %w", worktreePath, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// DeleteBranch deletes a local git branch
func DeleteBranch(projectRoot, branch string) error {
	return run("git", "-C", projectRoot, "branch", "-D", branch)
}

// Prune cleans up stale worktree references
func Prune(projectRoot string) error {
	return run("git", "-C", projectRoot, "worktree", "prune")
}

func branchExists(projectRoot, branch string) bool {
	cmd := exec.Command("git", "-C", projectRoot, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return cmd.Run() == nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
