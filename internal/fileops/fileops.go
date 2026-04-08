package fileops

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"

	"treemux/internal/config"
)

func Apply(ops config.FileOps, sourceRoot, targetRoot string) error {
	if err := applyCopyOps(ops.Copy, sourceRoot, targetRoot); err != nil {
		return err
	}
	if err := applySymlinkOps(ops.Symlink, sourceRoot, targetRoot); err != nil {
		return err
	}
	return nil
}

func applyCopyOps(patterns []string, sourceRoot, targetRoot string) error {
	for _, match := range expandPatterns(sourceRoot, patterns) {
		rel, err := filepath.Rel(sourceRoot, match)
		if err != nil {
			return fmt.Errorf("resolve copy relative path for %s: %w", match, err)
		}

		destination := filepath.Join(targetRoot, rel)
		if err := os.RemoveAll(destination); err != nil {
			return fmt.Errorf("clear copy destination %s: %w", destination, err)
		}
		if err := copyPath(match, destination); err != nil {
			return err
		}
	}
	return nil
}

func applySymlinkOps(patterns []string, sourceRoot, targetRoot string) error {
	for _, match := range expandPatterns(sourceRoot, patterns) {
		rel, err := filepath.Rel(sourceRoot, match)
		if err != nil {
			return fmt.Errorf("resolve symlink relative path for %s: %w", match, err)
		}

		destination := filepath.Join(targetRoot, rel)
		destinationParent := filepath.Dir(destination)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return fmt.Errorf("create symlink parent for %s: %w", destination, err)
		}
		if err := os.RemoveAll(destination); err != nil {
			return fmt.Errorf("clear symlink destination %s: %w", destination, err)
		}

		relativeSource, err := filepath.Rel(destinationParent, match)
		if err != nil {
			return fmt.Errorf("resolve relative symlink path for %s: %w", match, err)
		}

		if err := os.Symlink(relativeSource, destination); err != nil {
			return fmt.Errorf("symlink %s -> %s: %w", destination, relativeSource, err)
		}
	}
	return nil
}

func expandPatterns(root string, patterns []string) []string {
	seen := map[string]struct{}{}
	var matches []string

	for _, pattern := range patterns {
		glob := filepath.Join(root, filepath.FromSlash(pattern))
		expanded, err := doublestar.FilepathGlob(glob)
		if err != nil {
			continue
		}
		for _, match := range expanded {
			if _, ok := seen[match]; ok {
				continue
			}
			seen[match] = struct{}{}
			matches = append(matches, match)
		}
	}

	return matches
}

func copyPath(source, destination string) error {
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("stat %s: %w", source, err)
	}

	if info.IsDir() {
		return copyDir(source, destination, info.Mode())
	}
	return copyFile(source, destination, info.Mode())
}

func copyDir(source, destination string, mode fs.FileMode) error {
	if err := os.MkdirAll(destination, mode.Perm()); err != nil {
		return fmt.Errorf("mkdir %s: %w", destination, err)
	}

	entries, err := os.ReadDir(source)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", source, err)
	}

	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		destinationPath := filepath.Join(destination, entry.Name())
		if err := copyPath(sourcePath, destinationPath); err != nil {
			return err
		}
	}

	return nil
}

func copyFile(source, destination string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(destination), err)
	}

	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer input.Close()

	output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return fmt.Errorf("create %s: %w", destination, err)
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return fmt.Errorf("copy %s to %s: %w", source, destination, err)
	}

	return nil
}
