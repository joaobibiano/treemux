package treemux

import (
	"path/filepath"
	"regexp"
	"strings"
)

var invalidHandleChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
var repeatedDashes = regexp.MustCompile(`-+`)

func DeriveHandle(branch, naming, prefix string) string {
	return DeriveHandleWithName(branch, "", naming, prefix)
}

func DeriveHandleWithName(branch, explicitName, naming, prefix string) string {
	if explicitName != "" {
		return slugify(explicitName)
	}

	base := branch
	if naming == "basename" {
		parts := strings.Split(branch, "/")
		base = parts[len(parts)-1]
	}

	handle := slugify(base)
	if prefix != "" {
		handle = prefix + handle
	}
	return handle
}

func ResolveWorktreeDir(repoRoot, configured string) string {
	if configured == "" {
		return filepath.Join(filepath.Dir(repoRoot), filepath.Base(repoRoot)+"__worktrees")
	}
	if filepath.IsAbs(configured) {
		return filepath.Clean(configured)
	}
	return filepath.Join(repoRoot, configured)
}

func WindowName(prefix, handle string) string {
	return prefix + handle
}

func slugify(value string) string {
	value = strings.ReplaceAll(value, string(filepath.Separator), "-")
	value = strings.ReplaceAll(value, "/", "-")
	value = invalidHandleChars.ReplaceAllString(value, "-")
	value = repeatedDashes.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	value = strings.ToLower(value)
	if value == "" {
		return "worktree"
	}
	return value
}
