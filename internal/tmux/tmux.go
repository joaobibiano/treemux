package tmux

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"treemux/internal/config"
)

type Window struct {
	Target string
	Name   string
	Handle string
	Path   string
}

func CurrentSession() (string, error) {
	if os.Getenv("TMUX") == "" {
		return "", errors.New("treemux add must run inside tmux")
	}

	output, err := run("display-message", "-p", "#S")
	if err != nil {
		return "", fmt.Errorf("resolve current tmux session: %w", err)
	}

	return strings.TrimSpace(output), nil
}

func CreateWindow(session, name, cwd string, background bool) (target string, paneID string, err error) {
	args := []string{"new-window", "-P", "-F", "#{session_name}:#{window_index}\t#{pane_id}", "-t", session, "-n", name, "-c", cwd}
	if background {
		args = append(args, "-d")
	}

	output, err := run(args...)
	if err != nil {
		return "", "", fmt.Errorf("create tmux window: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(output), "\t")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected tmux window output %q", output)
	}

	return parts[0], parts[1], nil
}

func SplitPane(windowTarget, cwd string, pane config.Pane) (string, error) {
	args := []string{"split-window", "-P", "-F", "#{pane_id}", "-t", windowTarget, "-c", cwd, "-d"}

	if splitFlag := tmuxSplitFlag(pane.Split); splitFlag != "" {
		args = append(args, splitFlag)
	}

	if pane.Percentage > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", pane.Percentage))
	} else if pane.Size > 0 {
		args = append(args, "-l", fmt.Sprintf("%d", pane.Size))
	}

	output, err := run(args...)
	if err != nil {
		return "", fmt.Errorf("split tmux pane: %w", err)
	}

	return strings.TrimSpace(output), nil
}

func tmuxSplitFlag(split string) string {
	switch split {
	case "horizontal":
		return "-h"
	case "vertical":
		return "-v"
	default:
		return ""
	}
}

func SetWindowMetadata(windowTarget, handle, path string) error {
	if _, err := run("set-option", "-wq", "-t", windowTarget, "@treemux_handle", handle); err != nil {
		return fmt.Errorf("set tmux handle metadata: %w", err)
	}
	if _, err := run("set-option", "-wq", "-t", windowTarget, "@treemux_path", path); err != nil {
		return fmt.Errorf("set tmux path metadata: %w", err)
	}
	return nil
}

func SendCommand(paneID, command string) error {
	if _, err := run("send-keys", "-t", paneID, command, "C-m"); err != nil {
		return fmt.Errorf("send command to pane %s: %w", paneID, err)
	}
	return nil
}

func SelectPane(paneID string, zoom bool) error {
	if _, err := run("select-pane", "-t", paneID); err != nil {
		return fmt.Errorf("select pane %s: %w", paneID, err)
	}
	if zoom {
		if _, err := run("resize-pane", "-Z", "-t", paneID); err != nil {
			return fmt.Errorf("zoom pane %s: %w", paneID, err)
		}
	}
	return nil
}

func ListWindows() ([]Window, error) {
	output, err := run("list-windows", "-a", "-F", "#{session_name}:#{window_index}\t#{window_name}\t#{@treemux_handle}\t#{@treemux_path}")
	if err != nil {
		return nil, fmt.Errorf("list tmux windows: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	windows := make([]Window, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}
		windows = append(windows, Window{
			Target: parts[0],
			Name:   parts[1],
			Handle: parts[2],
			Path:   parts[3],
		})
	}

	return windows, nil
}

func FindWindow(handle, path, windowName string) (*Window, error) {
	windows, err := ListWindows()
	if err != nil {
		return nil, err
	}

	for _, window := range windows {
		if window.Handle == handle || window.Path == path {
			return &window, nil
		}
	}

	for _, window := range windows {
		if window.Name == windowName {
			return &window, nil
		}
	}

	return nil, nil
}

func KillWindow(target string) error {
	if _, err := run("kill-window", "-t", target); err != nil {
		return fmt.Errorf("kill tmux window %s: %w", target, err)
	}
	return nil
}

func run(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)

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
