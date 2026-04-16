package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"treemux/internal/treemux"
)

func main() {
	service := treemux.Service{}

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error

	switch os.Args[1] {
	case "add":
		err = runAdd(service, os.Args[2:])
	case "remove", "rm":
		err = runRemove(service, os.Args[2:])
	case "join":
		err = runJoin(service, os.Args[2:])
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "treemux: %v\n", err)
		os.Exit(1)
	}
}

func runAdd(service treemux.Service, args []string) error {
	args = reorderInterspersedFlags(args, map[string]bool{
		"--base":         true,
		"--name":         true,
		"--background":   false,
		"--no-hooks":     false,
		"--no-file-ops":  false,
		"--no-pane-cmds": false,
	})

	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var opts treemux.AddOptions
	fs.StringVar(&opts.Base, "base", "", "base branch/commit/tag")
	fs.StringVar(&opts.Name, "name", "", "custom worktree handle")
	fs.BoolVar(&opts.Background, "background", false, "create tmux window without switching to it")
	fs.BoolVar(&opts.NoHooks, "no-hooks", false, "skip post_create hooks")
	fs.BoolVar(&opts.NoFileOps, "no-file-ops", false, "skip file copy/symlink operations")
	fs.BoolVar(&opts.NoPaneCmds, "no-pane-cmds", false, "skip pane commands")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("usage: treemux add <branch> [options]")
	}

	return service.Add(fs.Arg(0), opts)
}

func runRemove(service treemux.Service, args []string) error {
	args = reorderInterspersedFlags(args, map[string]bool{
		"--force":       false,
		"--keep-branch": false,
	})

	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var opts treemux.RemoveOptions
	fs.BoolVar(&opts.Force, "force", false, "force removal even with uncommitted changes")
	fs.BoolVar(&opts.KeepBranch, "keep-branch", false, "remove worktree/window but keep branch")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() > 1 {
		return fmt.Errorf("usage: treemux remove [name] [options]")
	}

	name := ""
	if fs.NArg() == 1 {
		name = fs.Arg(0)
	}

	return service.Remove(name, opts)
}

func runJoin(service treemux.Service, args []string) error {
	fs := flag.NewFlagSet("join", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("usage: treemux join <name>")
	}

	return service.Join(fs.Arg(0), treemux.JoinOptions{})
}

func reorderInterspersedFlags(args []string, knownFlags map[string]bool) []string {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			positionals = append(positionals, args[i:]...)
			break
		}

		name, hasInlineValue := splitFlagArg(arg)
		needsValue, isKnownFlag := knownFlags[name]
		if !isKnownFlag {
			positionals = append(positionals, arg)
			continue
		}

		flags = append(flags, arg)
		if needsValue && !hasInlineValue && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}

	return append(flags, positionals...)
}

func splitFlagArg(arg string) (string, bool) {
	if !strings.HasPrefix(arg, "-") {
		return arg, false
	}
	name, _, found := strings.Cut(arg, "=")
	return name, found
}

func usage() {
	fmt.Fprintf(os.Stderr, `treemux manages git worktrees and tmux windows.

Usage:
  treemux add <branch> [options]
  treemux remove [name] [options]
  treemux join <name>

Commands:
  add       Create a worktree and tmux window
  remove    Remove a worktree, tmux window, and branch
  rm        Alias for remove
  join      Select the tmux window for an existing worktree
`)
}
