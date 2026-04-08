package tmux

import "testing"

func TestTmuxSplitFlagMatchesWorkmuxSemantics(t *testing.T) {
	if got := tmuxSplitFlag("horizontal"); got != "-h" {
		t.Fatalf("expected horizontal split to use -h, got %q", got)
	}

	if got := tmuxSplitFlag("vertical"); got != "-v" {
		t.Fatalf("expected vertical split to use -v, got %q", got)
	}

	if got := tmuxSplitFlag(""); got != "" {
		t.Fatalf("expected empty split flag for unspecified split, got %q", got)
	}
}
