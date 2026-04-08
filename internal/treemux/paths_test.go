package treemux

import (
	"path/filepath"
	"testing"
)

func TestDeriveHandle(t *testing.T) {
	if got := DeriveHandle("feature/auth-flow", "full", ""); got != "feature-auth-flow" {
		t.Fatalf("unexpected full handle %q", got)
	}
	if got := DeriveHandle("feature/auth-flow", "basename", "wm-"); got != "wm-auth-flow" {
		t.Fatalf("unexpected basename handle %q", got)
	}
	if got := DeriveHandleWithName("feature/auth-flow", "My Custom Name", "basename", "wm-"); got != "My-Custom-Name" && got != "my-custom-name" {
		t.Fatalf("unexpected explicit handle %q", got)
	}
}

func TestResolveWorktreeDir(t *testing.T) {
	repoRoot := filepath.Join("/tmp", "repo")
	defaultDir := ResolveWorktreeDir(repoRoot, "")
	if defaultDir != filepath.Join("/tmp", "repo__worktrees") {
		t.Fatalf("unexpected default worktree dir %q", defaultDir)
	}

	relativeDir := ResolveWorktreeDir(repoRoot, ".worktrees")
	if relativeDir != filepath.Join(repoRoot, ".worktrees") {
		t.Fatalf("unexpected relative worktree dir %q", relativeDir)
	}
}
