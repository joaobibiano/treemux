package main

import (
	"reflect"
	"testing"
)

func TestReorderInterspersedFlagsForAdd(t *testing.T) {
	got := reorderInterspersedFlags([]string{
		"feature/test",
		"--no-hooks",
		"--base",
		"main",
		"--name=my-worktree",
	}, map[string]bool{
		"--base":        true,
		"--name":        true,
		"--no-hooks":    false,
		"--no-file-ops": false,
	})

	want := []string{"--no-hooks", "--base", "main", "--name=my-worktree", "feature/test"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected reordered args\n got: %#v\nwant: %#v", got, want)
	}
}

func TestReorderInterspersedFlagsForRemove(t *testing.T) {
	got := reorderInterspersedFlags([]string{
		"my-handle",
		"--keep-branch",
	}, map[string]bool{
		"--keep-branch": false,
		"--force":       false,
	})

	want := []string{"--keep-branch", "my-handle"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected reordered args\n got: %#v\nwant: %#v", got, want)
	}
}

func TestReorderInterspersedFlagsLeavesUnknownArgsInPlaceAsPositionals(t *testing.T) {
	got := reorderInterspersedFlags([]string{
		"feature/test",
		"--unknown",
	}, map[string]bool{
		"--no-hooks": false,
	})

	want := []string{"feature/test", "--unknown"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected reordered args\n got: %#v\nwant: %#v", got, want)
	}
}
