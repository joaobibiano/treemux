package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type Pane struct {
	Command    string `yaml:"command"`
	Focus      bool   `yaml:"focus"`
	Zoom       bool   `yaml:"zoom"`
	Split      string `yaml:"split"`
	Size       int    `yaml:"size"`
	Percentage int    `yaml:"percentage"`
}

type FileOps struct {
	Copy    []string
	Symlink []string
}

type Config struct {
	MainBranch     string
	BaseBranch     string
	WorktreeDir    string
	WorktreeNaming string
	WorktreePrefix string
	WindowPrefix   string
	Agent          string
	Panes          []Pane
	PostCreate     []string
	PreRemove      []string
	Files          FileOps
	ConfigDir      string
	ProjectConfig  string
	GlobalConfig   string
}

type rawConfig struct {
	MainBranch     *string    `yaml:"main_branch"`
	BaseBranch     *string    `yaml:"base_branch"`
	WorktreeDir    *string    `yaml:"worktree_dir"`
	WorktreeNaming *string    `yaml:"worktree_naming"`
	WorktreePrefix *string    `yaml:"worktree_prefix"`
	WindowPrefix   *string    `yaml:"window_prefix"`
	Agent          *string    `yaml:"agent"`
	Panes          *[]Pane    `yaml:"panes"`
	PostCreate     *[]string  `yaml:"post_create"`
	PreRemove      *[]string  `yaml:"pre_remove"`
	Files          rawFileOps `yaml:"files"`
}

type rawFileOps struct {
	Copy    *[]string `yaml:"copy"`
	Symlink *[]string `yaml:"symlink"`
}

func Load(repoRoot, startDir string) (Config, error) {
	repoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return Config{}, fmt.Errorf("resolve repo root: %w", err)
	}

	startDir, err = filepath.Abs(startDir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve start dir: %w", err)
	}

	globalPath, globalRaw, err := loadOptional(globalConfigPath())
	if err != nil {
		return Config{}, err
	}

	projectPath, err := findProjectConfig(startDir, repoRoot)
	if err != nil {
		return Config{}, err
	}

	projectRaw := rawConfig{}
	if projectPath != "" {
		_, projectRaw, err = loadOptional(projectPath)
		if err != nil {
			return Config{}, err
		}
	}

	configDir := repoRoot
	if projectPath != "" {
		configDir = filepath.Dir(projectPath)
	}

	cfg := Config{
		MainBranch:     mergeString(projectRaw.MainBranch, globalRaw.MainBranch),
		BaseBranch:     mergeString(projectRaw.BaseBranch, globalRaw.BaseBranch),
		WorktreeDir:    mergeString(projectRaw.WorktreeDir, globalRaw.WorktreeDir),
		WorktreeNaming: mergeString(projectRaw.WorktreeNaming, globalRaw.WorktreeNaming),
		WorktreePrefix: mergeString(projectRaw.WorktreePrefix, globalRaw.WorktreePrefix),
		WindowPrefix:   mergeString(projectRaw.WindowPrefix, globalRaw.WindowPrefix),
		Agent:          mergeString(projectRaw.Agent, globalRaw.Agent),
		Panes:          mergePanes(projectRaw.Panes, globalRaw.Panes),
		PostCreate:     mergeList(projectRaw.PostCreate, globalRaw.PostCreate),
		PreRemove:      mergeList(projectRaw.PreRemove, globalRaw.PreRemove),
		Files: FileOps{
			Copy:    mergeList(projectRaw.Files.Copy, globalRaw.Files.Copy),
			Symlink: mergeList(projectRaw.Files.Symlink, globalRaw.Files.Symlink),
		},
		ConfigDir:     configDir,
		ProjectConfig: projectPath,
		GlobalConfig:  globalPath,
	}

	if cfg.WorktreeNaming == "" {
		cfg.WorktreeNaming = "full"
	}
	if cfg.WindowPrefix == "" {
		cfg.WindowPrefix = "wm-"
	}
	if cfg.Agent == "" {
		cfg.Agent = "claude"
	}

	return cfg, nil
}

func mergeString(project, global *string) string {
	if project != nil {
		return *project
	}
	if global != nil {
		return *global
	}
	return ""
}

func mergePanes(project, global *[]Pane) []Pane {
	if project != nil {
		return slices.Clone(*project)
	}
	if global != nil {
		return slices.Clone(*global)
	}
	return nil
}

func mergeList(project, global *[]string) []string {
	if project == nil {
		if global == nil {
			return nil
		}
		return slices.Clone(*global)
	}

	merged := make([]string, 0, len(*project))
	globalItems := []string{}
	if global != nil {
		globalItems = *global
	}

	for _, item := range *project {
		if item == "<global>" {
			merged = append(merged, globalItems...)
			continue
		}
		merged = append(merged, item)
	}

	return merged
}

func loadOptional(path string) (string, rawConfig, error) {
	if path == "" {
		return "", rawConfig{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", rawConfig{}, nil
		}
		return "", rawConfig{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg rawConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", rawConfig{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	return path, cfg, nil
}

func globalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "workmux", "config.yaml")
}

func findProjectConfig(startDir, repoRoot string) (string, error) {
	current := startDir
	for {
		candidate := filepath.Join(current, ".workmux.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stat %s: %w", candidate, err)
		}

		if sameDir(current, repoRoot) {
			return "", nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", nil
		}
		current = parent
	}
}

func sameDir(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func (c Config) HasProjectConfig() bool {
	return strings.TrimSpace(c.ProjectConfig) != ""
}
