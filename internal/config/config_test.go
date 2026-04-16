package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMergesProjectAndGlobalConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}

	globalDir := filepath.Join(home, ".config", "workmux")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("mkdir global dir: %v", err)
	}

	globalConfig := `
window_prefix: global-
agent: codex
post_create:
  - global-hook
pre_remove:
  - global-remove
files:
  copy:
    - .env
`
	if err := os.WriteFile(filepath.Join(globalDir, "config.yaml"), []byte(globalConfig), 0o644); err != nil {
		t.Fatalf("write global config: %v", err)
	}

	repoRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repoRoot, "nested", "deeper"), 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	projectConfig := `
main_branch: master
worktree_naming: basename
post_create:
  - <global>
  - project-hook
pre_remove: []
files:
  copy:
    - <global>
    - .env.local
`
	projectPath := filepath.Join(repoRoot, ".workmux.yaml")
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg, err := Load(repoRoot, filepath.Join(repoRoot, "nested", "deeper"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.MainBranch != "master" {
		t.Fatalf("expected main branch master, got %q", cfg.MainBranch)
	}
	if cfg.WorktreeNaming != "basename" {
		t.Fatalf("expected basename naming, got %q", cfg.WorktreeNaming)
	}
	if cfg.WindowPrefix != "global-" {
		t.Fatalf("expected global window prefix, got %q", cfg.WindowPrefix)
	}
	if cfg.Agent != "codex" {
		t.Fatalf("expected global agent codex, got %q", cfg.Agent)
	}

	expectedPostCreate := []string{"global-hook", "project-hook"}
	assertStringSlice(t, cfg.PostCreate, expectedPostCreate)
	assertStringSlice(t, cfg.PreRemove, []string{})
	assertStringSlice(t, cfg.Files.Copy, []string{".env", ".env.local"})

	if cfg.ConfigDir != repoRoot {
		t.Fatalf("expected config dir %q, got %q", repoRoot, cfg.ConfigDir)
	}
	if cfg.ProjectConfig != projectPath {
		t.Fatalf("expected project config path %q, got %q", projectPath, cfg.ProjectConfig)
	}
}

func TestLoadAppliesDefaultsWithoutConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	repoRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	cfg, err := Load(repoRoot, repoRoot)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.WorktreeNaming != "full" {
		t.Fatalf("expected default naming full, got %q", cfg.WorktreeNaming)
	}
	if cfg.WindowPrefix != "" {
		t.Fatalf("expected empty default window prefix, got %q", cfg.WindowPrefix)
	}
	if cfg.Agent != "claude" {
		t.Fatalf("expected default agent claude, got %q", cfg.Agent)
	}
	if cfg.ConfigDir != repoRoot {
		t.Fatalf("expected config dir %q, got %q", repoRoot, cfg.ConfigDir)
	}
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice length mismatch: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice mismatch: got %v want %v", got, want)
		}
	}
}
