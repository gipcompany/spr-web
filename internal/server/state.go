package server

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gipcompany/spr-web/internal/sprbridge"
)

// EditSession describes an in-progress `spr edit` session (state owned by
// spr itself via the .git/spr_edit_state file).
type EditSession struct {
	CommitID string `json:"commitId"`
	Subject  string `json:"subject"`
	// Conflict is true while the rebase is stopped on a conflict
	// (.git/REBASE_HEAD exists), as opposed to the initial edit stop.
	Conflict bool `json:"conflict"`
}

// RepoState is cheap, file-system derived repository state. It is computed
// on demand by the server itself (no child process): plain stat calls plus
// one `git status --porcelain`.
type RepoState struct {
	EditSession *EditSession `json:"editSession"`
	// RebaseInProgress is true whenever a git rebase is underway, including
	// edit sessions. A rebase without an edit session means something
	// outside spr-web (or an interrupted run) left the repository mid-rebase
	// and needs `git rebase --abort` in a terminal.
	RebaseInProgress bool                        `json:"rebaseInProgress"`
	WorkingTree      sprbridge.WorkingTreeCounts `json:"workingTree"`
}

func (s *RepoState) editing() bool { return s.EditSession != nil }

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// detectRepoState inspects the git dir and working tree.
func detectRepoState(repoRoot, gitDir string) RepoState {
	state := RepoState{}

	state.RebaseInProgress = exists(filepath.Join(gitDir, "rebase-merge")) ||
		exists(filepath.Join(gitDir, "rebase-apply"))

	if data, err := os.ReadFile(filepath.Join(gitDir, "spr_edit_state")); err == nil {
		session := &EditSession{
			Conflict: exists(filepath.Join(gitDir, "REBASE_HEAD")),
		}
		for _, line := range strings.Split(string(data), "\n") {
			if v, ok := strings.CutPrefix(line, "commit_id="); ok {
				session.CommitID = strings.TrimSpace(v)
			}
			if v, ok := strings.CutPrefix(line, "commit_subject="); ok {
				session.Subject = strings.TrimSpace(v)
			}
		}
		state.EditSession = session
	}

	if out, err := gitOutput(repoRoot, "status", "--porcelain"); err == nil {
		state.WorkingTree = sprbridge.CountWorkingTree(out)
	}

	return state
}

// gitOutput runs a read-only git command in dir. The server intentionally
// avoids spr's realgit here: realgit exits the process on failure, which
// must never happen inside the server.
func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}
