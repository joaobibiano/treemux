package treemux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"treemux/internal/cleanup"
	"treemux/internal/config"
	"treemux/internal/fileops"
	"treemux/internal/gitutil"
	"treemux/internal/tmux"
)

type Service struct{}

type AddOptions struct {
	Base       string
	Name       string
	Background bool
	NoHooks    bool
	NoFileOps  bool
	NoPaneCmds bool
}

type RemoveOptions struct {
	Force      bool
	KeepBranch bool
}

type JoinOptions struct{}

func (Service) Add(branch string, opts AddOptions) error {
	startDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current directory: %w", err)
	}

	currentRoot, err := gitutil.RepoRoot(startDir)
	if err != nil {
		return err
	}

	worktrees, err := gitutil.WorktreeList(currentRoot)
	if err != nil {
		return err
	}

	mainRoot := mainWorktreeRoot(worktrees, currentRoot)

	cfg, err := config.Load(currentRoot, startDir)
	if err != nil {
		return err
	}
	sourceConfigDir := mapConfigDir(currentRoot, mainRoot, cfg.ConfigDir)

	session, err := tmux.CurrentSession()
	if err != nil {
		return err
	}

	handle := DeriveHandleWithName(branch, opts.Name, cfg.WorktreeNaming, cfg.WorktreePrefix)
	for _, worktree := range worktrees {
		if filepath.Base(worktree.Path) == handle {
			return fmt.Errorf("worktree handle %q already exists", handle)
		}
		if worktree.Branch == branch {
			return fmt.Errorf("branch %q is already checked out in %s", branch, worktree.Path)
		}
	}

	worktreeDir := ResolveWorktreeDir(mainRoot, cfg.WorktreeDir)
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		return fmt.Errorf("create worktree directory %s: %w", worktreeDir, err)
	}

	worktreePath := filepath.Join(worktreeDir, handle)
	base, err := resolveBaseBranch(mainRoot, startDir, cfg, opts)
	if err != nil {
		return err
	}

	branchExisted, err := gitutil.LocalBranchExists(mainRoot, branch)
	if err != nil {
		return err
	}

	if err := gitutil.AddWorktree(mainRoot, worktreePath, branch, base); err != nil {
		return err
	}

	cleanupOnError := true
	defer func() {
		if cleanupOnError {
			_ = gitutil.RemoveWorktree(mainRoot, worktreePath, true)
			if !branchExisted {
				_ = gitutil.DeleteBranch(mainRoot, branch, true)
			}
		}
	}()

	worktreeConfigDir := mapConfigDir(currentRoot, worktreePath, cfg.ConfigDir)
	effectiveWorkingDir := existingDirOrFallback(worktreeConfigDir, worktreePath)

	if !opts.NoFileOps {
		if err := fileops.Apply(cfg.Files, sourceConfigDir, worktreePath); err != nil {
			return err
		}
	}

	env := worktreeEnv(worktreeConfigDir, mainRoot, worktreePath, handle, branch)
	if !opts.NoHooks {
		if err := runCommands(cfg.PostCreate, effectiveWorkingDir, env); err != nil {
			return fmt.Errorf("run post_create hooks: %w", err)
		}
	}

	windowName := WindowName(cfg.WindowPrefix, handle)
	windowTarget, firstPaneID, err := tmux.CreateWindow(session, windowName, effectiveWorkingDir, opts.Background)
	if err != nil {
		return err
	}
	if err := tmux.SetWindowMetadata(windowTarget, handle, worktreePath); err != nil {
		return err
	}

	panes := cfg.Panes
	if len(panes) == 0 {
		panes = []config.Pane{{Focus: true}}
	}

	paneIDs := make([]string, len(panes))
	paneIDs[0] = firstPaneID
	for i := 1; i < len(panes); i++ {
		paneID, err := tmux.SplitPane(windowTarget, effectiveWorkingDir, panes[i])
		if err != nil {
			return err
		}
		paneIDs[i] = paneID
	}

	focusPaneID := firstPaneID
	focusZoom := false
	for i, pane := range panes {
		if !opts.NoPaneCmds && strings.TrimSpace(pane.Command) != "" {
			command := composePaneCommand(env, strings.ReplaceAll(pane.Command, "<agent>", cfg.Agent))
			if err := tmux.SendCommand(paneIDs[i], command); err != nil {
				return err
			}
		}
		if pane.Focus || pane.Zoom {
			focusPaneID = paneIDs[i]
			focusZoom = pane.Zoom
		}
	}

	if err := tmux.SelectPane(focusPaneID, focusZoom); err != nil {
		return err
	}

	cleanupOnError = false
	return nil
}

func (Service) Remove(name string, opts RemoveOptions) error {
	startDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current directory: %w", err)
	}

	currentRoot, err := gitutil.RepoRoot(startDir)
	if err != nil {
		return err
	}

	if err := gitutil.PruneWorktrees(currentRoot); err != nil {
		return err
	}

	worktrees, err := gitutil.WorktreeList(currentRoot)
	if err != nil {
		return err
	}

	mainRoot := mainWorktreeRoot(worktrees, currentRoot)

	cfg, err := config.Load(currentRoot, startDir)
	if err != nil {
		return err
	}

	target, err := gitutil.MatchWorktree(worktrees, name, startDir)
	if err != nil {
		return err
	}
	if target.IsMain {
		return fmt.Errorf("refusing to remove the main worktree %s", target.Path)
	}

	dirty, err := gitutil.IsDirty(target.Path)
	if err != nil {
		return err
	}
	if dirty && !opts.Force {
		return fmt.Errorf("worktree %q has uncommitted changes; rerun with --force to remove it", target.Path)
	}

	handle := filepath.Base(target.Path)
	targetConfigDir := mapConfigDir(currentRoot, target.Path, cfg.ConfigDir)
	effectiveWorkingDir := existingDirOrFallback(targetConfigDir, target.Path)
	env := worktreeEnv(targetConfigDir, mainRoot, target.Path, handle, target.Branch)
	if err := runCommands(cfg.PreRemove, effectiveWorkingDir, env); err != nil {
		if !opts.Force {
			return fmt.Errorf("run pre_remove hooks: %w", err)
		}
		fmt.Fprintf(os.Stderr, "treemux: pre_remove hooks failed, continuing due to --force: %v\n", err)
	}

	if cleanup.LooksLikeNodeProject(target.Path) {
		if err := cleanup.RemoveNodeModules(target.Path); err != nil {
			fmt.Fprintf(os.Stderr, "treemux: node_modules cleanup failed, continuing: %v\n", err)
		}
	}

	windowName := WindowName(cfg.WindowPrefix, handle)
	window, err := tmux.FindWindow(handle, target.Path, windowName)
	if err != nil {
		return err
	}

	if err := gitutil.RemoveWorktree(mainRoot, target.Path, opts.Force); err != nil {
		return err
	}

	if !opts.KeepBranch && target.Branch != "" {
		if err := gitutil.DeleteBranch(mainRoot, target.Branch, opts.Force); err != nil {
			return fmt.Errorf("removed worktree but failed to delete branch %q: %w", target.Branch, err)
		}
	}

	if window != nil {
		if err := tmux.KillWindow(window.Target); err != nil {
			return err
		}
	}

	return nil
}

func (Service) Join(name string, _ JoinOptions) error {
	startDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current directory: %w", err)
	}

	currentRoot, err := gitutil.RepoRoot(startDir)
	if err != nil {
		return err
	}

	worktrees, err := gitutil.WorktreeList(currentRoot)
	if err != nil {
		return err
	}

	cfg, err := config.Load(currentRoot, startDir)
	if err != nil {
		return err
	}

	target, err := gitutil.MatchWorktree(worktrees, name, startDir)
	if err != nil {
		return err
	}

	if _, err := tmux.CurrentSession(); err != nil {
		return err
	}

	handle := filepath.Base(target.Path)
	windowName := WindowName(cfg.WindowPrefix, handle)
	window, err := tmux.FindWindow(handle, target.Path, windowName)
	if err != nil {
		return err
	}
	if window == nil {
		return fmt.Errorf("no tmux window found for worktree %q; run `treemux add` or create a window manually", handle)
	}

	return tmux.SelectWindow(window.Target)
}

func resolveBaseBranch(repoRoot, startDir string, cfg config.Config, opts AddOptions) (string, error) {
	if opts.Base != "" {
		return opts.Base, nil
	}
	if cfg.BaseBranch != "" {
		return cfg.BaseBranch, nil
	}
	branch, err := gitutil.CurrentBranch(startDir)
	if err != nil {
		return "", err
	}
	if branch != "" {
		return branch, nil
	}
	if cfg.MainBranch != "" {
		return cfg.MainBranch, nil
	}
	return gitutil.DefaultMainBranch(repoRoot)
}

func mainWorktreeRoot(worktrees []gitutil.Worktree, fallback string) string {
	for _, worktree := range worktrees {
		if worktree.IsMain {
			return worktree.Path
		}
	}
	return fallback
}

func mapConfigDir(fromRoot, toRoot, configDir string) string {
	if configDir == "" {
		return toRoot
	}

	rel, err := filepath.Rel(fromRoot, configDir)
	if err != nil || rel == "." {
		return toRoot
	}
	if strings.HasPrefix(rel, "..") {
		return toRoot
	}
	return filepath.Join(toRoot, rel)
}

func existingDirOrFallback(path, fallback string) string {
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return path
	}
	return fallback
}

func worktreeEnv(configDir, repoRoot, worktreePath, handle, branch string) map[string]string {
	return map[string]string{
		"WM_BRANCH_NAME":   branch,
		"WM_CONFIG_DIR":    configDir,
		"WM_HANDLE":        handle,
		"WM_PROJECT_ROOT":  repoRoot,
		"WM_WORKTREE_PATH": worktreePath,
	}
}

func runCommands(commands []string, dir string, env map[string]string) error {
	for _, command := range commands {
		if strings.TrimSpace(command) == "" {
			continue
		}

		cmd := exec.Command("sh", "-lc", command)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), formatEnv(env)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			message := strings.TrimSpace(string(output))
			if message != "" {
				return fmt.Errorf("%s: %w", message, err)
			}
			return err
		}
	}

	return nil
}

func composePaneCommand(env map[string]string, command string) string {
	parts := make([]string, 0, len(env))
	for _, item := range formatEnv(env) {
		parts = append(parts, "export "+item)
	}
	parts = append(parts, command)
	return strings.Join(parts, "; ")
}

func formatEnv(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	formatted := make([]string, 0, len(keys))
	for _, key := range keys {
		formatted = append(formatted, key+"="+shellQuote(env[key]))
	}
	return formatted
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
