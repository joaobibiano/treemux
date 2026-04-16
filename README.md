# treemux

`treemux` is a small tmux-only wrapper around `git worktree`.

It is meant for the narrow workflow of:

- creating a worktree and matching tmux window
- removing the worktree, tmux window, and optionally the branch

It reads the existing WorkMux config shape so it can work in repos that already
use `.workmux.yaml`.

## Scope

Current commands:

- `treemux add`
- `treemux remove`
- `treemux rm`
- `treemux join`

Current config support:

- `main_branch`
- `base_branch`
- `worktree_dir`
- `worktree_naming`
- `worktree_prefix`
- `window_prefix`
- `agent`
- `panes`
- `post_create`
- `pre_remove`
- `files.copy`
- `files.symlink`

Current constraints:

- tmux only
- no dashboard
- no merge/open/resurrect flow

## Install

From this repo:

```sh
./bin/install
```

By default this installs to `~/.config/bin/treemux`.

Override the destination with:

```sh
TREEMUX_INSTALL_DIR=/some/path ./bin/install
```

## Usage

Create a worktree:

```sh
treemux add my-feature
treemux add my-feature --base main
treemux add my-feature --name short-handle
treemux add my-feature --no-hooks --no-file-ops --no-pane-cmds
```

Remove a worktree and branch:

```sh
treemux remove my-feature
```

Keep the branch but remove the worktree and tmux window:

```sh
treemux remove my-feature --keep-branch
```

Force removal even with uncommitted changes:

```sh
treemux remove my-feature --force
```

Flags can be placed before or after the positional argument.

Join the tmux window of an existing worktree:

```sh
treemux join my-feature
```

Accepts the worktree handle, branch name, or full path.

## Config Example

```yaml
main_branch: master

panes:
  - command: <agent>
    focus: true
  - command: pnpm install
    split: horizontal
    size: 50
```

## Behavior Notes

- `treemux add` creates a git worktree and a tmux window in the current tmux
  session.
- `treemux remove` removes the tmux window, removes the worktree, and deletes
  the local branch.
- `treemux remove --keep-branch` keeps the local branch.
- `pre_remove` is optional.
- built-in `node_modules` cleanup still runs on remove for Node-style repos.
- explicit `--name` bypasses `worktree_prefix`.
- tmux window names are unprefixed by default. Set `window_prefix` if you want one.

## Why

The main goal is to keep the workflow simple and stateless:

- Git is the source of truth for worktrees
- tmux is the source of truth for windows
- `.workmux.yaml` remains the source of truth for local setup
