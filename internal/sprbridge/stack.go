package sprbridge

import (
	"strings"

	"github.com/ejoffe/spr/git"
)

// ResolveCommitIndex finds the zero-based stack index of the commit whose
// hash starts with target. It returns -1 when the commit is not in the stack
// (the stack changed between UI display and execution).
func ResolveCommitIndex(commits []git.Commit, target string) int {
	if target == "" {
		return -1
	}
	for i, c := range commits {
		if strings.HasPrefix(c.CommitHash, target) {
			return i
		}
	}
	return -1
}

// HasStagedChanges reports whether `git status --porcelain` output contains
// at least one staged entry. spr's amend runs `git commit --fixup`, which
// fails without staged changes, so this is checked up front.
func HasStagedChanges(porcelain string) bool {
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) >= 2 && line[0] != ' ' && line[0] != '?' && line[0] != '!' {
			return true
		}
	}
	return false
}

// WorkingTreeCounts summarizes `git status --porcelain` output.
type WorkingTreeCounts struct {
	Staged    int `json:"staged"`
	Unstaged  int `json:"unstaged"`
	Untracked int `json:"untracked"`
}

// CountWorkingTree parses `git status --porcelain` (v1) output.
func CountWorkingTree(porcelain string) WorkingTreeCounts {
	var counts WorkingTreeCounts
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) < 2 {
			continue
		}
		x, y := line[0], line[1]
		if x == '?' {
			counts.Untracked++
			continue
		}
		if x != ' ' && x != '!' {
			counts.Staged++
		}
		if y != ' ' && y != '!' {
			counts.Unstaged++
		}
	}
	return counts
}
