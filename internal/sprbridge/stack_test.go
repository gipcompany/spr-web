package sprbridge

import (
	"testing"

	"github.com/ejoffe/spr/git"
)

func stack() []git.Commit {
	return []git.Commit{
		{CommitID: "id-1", CommitHash: "aaaa1111aaaa1111", Subject: "first"},
		{CommitID: "id-2", CommitHash: "bbbb2222bbbb2222", Subject: "second"},
		{CommitID: "id-3", CommitHash: "cccc3333cccc3333", Subject: "third"},
	}
}

func TestResolveCommitIndex(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   int
	}{
		{"full hash", "bbbb2222bbbb2222", 1},
		{"prefix", "cccc", 2},
		{"bottom of stack", "aaaa1111", 0},
		{"not found", "dddd", -1},
		{"empty target", "", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveCommitIndex(stack(), tt.target); got != tt.want {
				t.Errorf("ResolveCommitIndex(%q) = %d, want %d", tt.target, got, tt.want)
			}
		})
	}
}

func TestHasStagedChanges(t *testing.T) {
	tests := []struct {
		name      string
		porcelain string
		want      bool
	}{
		{"empty", "", false},
		{"staged modification", "M  file.go\n", true},
		{"staged addition", "A  file.go\n", true},
		{"staged rename", "R  old -> new\n", true},
		{"unstaged only", " M file.go\n", false},
		{"untracked only", "?? file.go\n", false},
		{"mixed", "?? junk\n M dirty.go\nA  staged.go\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasStagedChanges(tt.porcelain); got != tt.want {
				t.Errorf("HasStagedChanges(%q) = %v, want %v", tt.porcelain, got, tt.want)
			}
		})
	}
}

func TestCountWorkingTree(t *testing.T) {
	porcelain := "M  staged.go\nMM both.go\n M unstaged.go\n?? new.txt\n?? other.txt\n"
	got := CountWorkingTree(porcelain)
	want := WorkingTreeCounts{Staged: 2, Unstaged: 2, Untracked: 2}
	if got != want {
		t.Errorf("CountWorkingTree = %+v, want %+v", got, want)
	}
}
