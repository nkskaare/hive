package worktree

import (
	"fmt"
	"io"
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

// Remove forcefully removes a git worktree.
// Falls back to rm + prune if the worktree's .git file was rewritten (e.g. by IsolateGitDir).
func Remove(worktreePath string) error {
	err := run("git", "worktree", "remove", "--force", worktreePath)
	if err != nil {
		// .git file may point to .hive/gitdir instead of .git/worktrees/<id>,
		// so git worktree remove may fail — fall back to manual cleanup.
		os.RemoveAll(worktreePath)
	}
	return nil
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

// IsolateGitDir creates a standalone git directory inside the worktree so the
// container can commit, branch and push without being able to modify the host
// repo's HEAD, config or refs.
//
// Layout created at <worktree>/.hive/gitdir/:
//   - HEAD, index          ← copied from .git/worktrees/<id>
//   - config, packed-refs  ← copied from .git/
//   - refs/                ← copied from .git/refs/
//   - objects/info/alternates → points to .git/objects (read-only)
//
// The worktree's .git file is rewritten to point to .hive/gitdir.
func IsolateGitDir(projectRoot, worktreePath, workerID string) error {
	gitDir := filepath.Join(projectRoot, ".git")
	wtGitDir := filepath.Join(gitDir, "worktrees", workerID)
	dst := filepath.Join(worktreePath, ".hive", "gitdir")

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("creating gitdir: %w", err)
	}

	// Copy worktree-specific state (HEAD, index, MERGE_HEAD, etc.)
	entries, err := os.ReadDir(wtGitDir)
	if err != nil {
		return fmt.Errorf("reading worktree gitdir: %w", err)
	}
	for _, e := range entries {
		name := e.Name()
		if name == "commondir" || name == "gitdir" {
			continue // skip — we're replacing these concepts
		}
		src := filepath.Join(wtGitDir, name)
		if e.IsDir() {
			if err := copyDir(src, filepath.Join(dst, name)); err != nil {
				return err
			}
		} else {
			if err := copyFile(src, filepath.Join(dst, name)); err != nil {
				return err
			}
		}
	}

	// Copy shared git state (config, refs, packed-refs, info)
	for _, name := range []string{"config", "packed-refs"} {
		src := filepath.Join(gitDir, name)
		if _, err := os.Stat(src); err == nil {
			if err := copyFile(src, filepath.Join(dst, name)); err != nil {
				return err
			}
		}
	}
	if err := copyDir(filepath.Join(gitDir, "refs"), filepath.Join(dst, "refs")); err != nil {
		return err
	}
	infoSrc := filepath.Join(gitDir, "info")
	if _, err := os.Stat(infoSrc); err == nil {
		if err := copyDir(infoSrc, filepath.Join(dst, "info")); err != nil {
			return err
		}
	}

	// Set up object store with alternates
	objInfoDir := filepath.Join(dst, "objects", "info")
	if err := os.MkdirAll(objInfoDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(objInfoDir, "alternates"),
		[]byte(filepath.Join(gitDir, "objects")+"\n"), 0o644); err != nil {
		return err
	}

	// Rewrite the worktree's .git file to point to the isolated gitdir
	return os.WriteFile(filepath.Join(worktreePath, ".git"),
		[]byte("gitdir: .hive/gitdir\n"), 0o644)
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
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
