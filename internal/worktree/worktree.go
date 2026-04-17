package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Exists checks whether a worktree directory already exists
func Exists(worktreePath string) bool {
	_, err := os.Stat(worktreePath)
	return err == nil
}

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

// CommitsAhead returns the number of commits the worktree branch is ahead of its upstream.
// Returns 0 if there is no upstream or on any error.
func CommitsAhead(worktreePath string) int {
	out, err := exec.Command("git", "-C", worktreePath, "rev-list", "--count", "@{upstream}..HEAD").Output()
	if err != nil {
		return 0
	}
	var n int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n)
	return n
}

// CommitCount returns the total number of commits on the worktree branch
// that are not on the default branch (main/master).
func CommitCount(projectRoot, worktreePath string) int {
	defaultBranch := getDefaultBranch(projectRoot)
	out, err := exec.Command("git", "-C", worktreePath, "rev-list", "--count", defaultBranch+"..HEAD").Output()
	if err != nil {
		return 0
	}
	var n int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n)
	return n
}

// LastCommitAge returns the time since the last commit in the worktree.
func LastCommitAge(worktreePath string) (time.Duration, bool) {
	out, err := exec.Command("git", "-C", worktreePath, "log", "-1", "--format=%ct").Output()
	if err != nil {
		return 0, false
	}
	var epoch int64
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &epoch)
	if epoch == 0 {
		return 0, false
	}
	return time.Since(time.Unix(epoch, 0)), true
}

func getDefaultBranch(projectRoot string) string {
	for _, name := range []string{"main", "master"} {
		cmd := exec.Command("git", "-C", projectRoot, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
		if cmd.Run() == nil {
			return name
		}
	}
	return "main"
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
