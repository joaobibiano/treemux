package gitutil

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Worktree struct {
	Path   string
	Head   string
	Branch string
	IsMain bool
}

func RepoRoot(startDir string) (string, error) {
	output, err := run(startDir, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("resolve git repo root: %w", err)
	}
	return strings.TrimSpace(output), nil
}

func CurrentBranch(dir string) (string, error) {
	output, err := run(dir, "git", "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolve current branch: %w", err)
	}
	branch := strings.TrimSpace(output)
	if branch == "HEAD" {
		return "", nil
	}
	return branch, nil
}

func DefaultMainBranch(repoRoot string) (string, error) {
	output, err := run(repoRoot, "git", "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		remoteRef := strings.TrimSpace(output)
		return strings.TrimPrefix(remoteRef, "origin/"), nil
	}

	for _, candidate := range []string{"main", "master"} {
		if err := branchExists(repoRoot, candidate); err == nil {
			return candidate, nil
		}
	}

	return "", errors.New("could not determine main branch")
}

func LocalBranchExists(repoRoot, branch string) (bool, error) {
	if err := branchExists(repoRoot, branch); err != nil {
		return false, nil
	}
	return true, nil
}

func branchExists(repoRoot, branch string) error {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = repoRoot
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func WorktreeList(repoRoot string) ([]Worktree, error) {
	output, err := run(repoRoot, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("list git worktrees: %w", err)
	}

	var worktrees []Worktree
	var current *Worktree
	index := 0

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if current != nil {
				current.IsMain = index == 0
				worktrees = append(worktrees, *current)
				index++
				current = nil
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			if current != nil {
				current.IsMain = index == 0
				worktrees = append(worktrees, *current)
				index++
			}
			current = &Worktree{Path: strings.TrimPrefix(line, "worktree ")}
			continue
		}

		if current == nil {
			continue
		}

		switch {
		case strings.HasPrefix(line, "HEAD "):
			current.Head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			branchRef := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(branchRef, "refs/heads/")
		case line == "detached":
			current.Branch = ""
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan worktree list: %w", err)
	}

	if current != nil {
		current.IsMain = index == 0
		worktrees = append(worktrees, *current)
	}

	return worktrees, nil
}

func AddWorktree(repoRoot, path, branch, base string) error {
	if err := branchExists(repoRoot, branch); err == nil {
		_, err := run(repoRoot, "git", "worktree", "add", path, branch)
		if err != nil {
			return fmt.Errorf("add existing branch worktree: %w", err)
		}
		return nil
	}

	args := []string{"git", "worktree", "add", "-b", branch, path}
	if base != "" {
		args = append(args, base)
	}

	_, err := run(repoRoot, args[0], args[1:]...)
	if err != nil {
		return fmt.Errorf("add new branch worktree: %w", err)
	}
	return nil
}

func PruneWorktrees(repoRoot string) error {
	if _, err := run(repoRoot, "git", "worktree", "prune"); err != nil {
		return fmt.Errorf("prune worktrees: %w", err)
	}
	return nil
}

func RemoveWorktree(repoRoot, path string, force bool) error {
	if force {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove worktree directory: %w", err)
		}
		return PruneWorktrees(repoRoot)
	}

	_, err := run(repoRoot, "git", "worktree", "remove", path)
	if err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}
	return nil
}

func DeleteBranch(repoRoot, branch string, force bool) error {
	args := []string{"git", "branch", "-d"}
	if force {
		args = []string{"git", "branch", "-D"}
	}
	args = append(args, branch)

	_, err := run(repoRoot, args[0], args[1:]...)
	if err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}
	return nil
}

func IsDirty(path string) (bool, error) {
	output, err := run(path, "git", "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("read git status: %w", err)
	}
	return strings.TrimSpace(output) != "", nil
}

func MatchWorktree(worktrees []Worktree, nameOrPath string, cwd string) (*Worktree, error) {
	if nameOrPath == "" {
		for _, worktree := range worktrees {
			if samePath(worktree.Path, cwd) {
				return &worktree, nil
			}
		}
		return nil, fmt.Errorf("current directory %q is not a registered git worktree", cwd)
	}

	for _, worktree := range worktrees {
		if samePath(worktree.Path, nameOrPath) || filepath.Base(worktree.Path) == nameOrPath || worktree.Branch == nameOrPath {
			return &worktree, nil
		}
	}

	return nil, fmt.Errorf("could not find worktree %q", nameOrPath)
}

func run(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message != "" {
			return "", fmt.Errorf("%w: %s", err, message)
		}
		return "", err
	}

	return stdout.String(), nil
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
