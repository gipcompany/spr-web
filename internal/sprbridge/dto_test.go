package sprbridge

import (
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
)

func testConfig() *config.Config {
	cfg := config.EmptyConfig()
	cfg.Repo.GitHubRepoOwner = "owner"
	cfg.Repo.GitHubRepoName = "repo"
	cfg.Repo.GitHubHost = "github.com"
	cfg.Repo.GitHubBranch = "main"
	cfg.Repo.RequireChecks = true
	cfg.Repo.RequireApproval = false
	return cfg
}

func TestBuildInfoPayload(t *testing.T) {
	cfg := testConfig()
	commits := stack()
	info := &github.GitHubInfo{
		UserName:    "user",
		LocalBranch: "feature",
		PullRequests: []*github.PullRequest{
			{
				ID:         "PR_1",
				Number:     101,
				FromBranch: "spr/main/id-1",
				ToBranch:   "main",
				Title:      "first",
				Commit:     commits[0],
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					ReviewApproved: false,
					NoConflicts:    true,
					Stacked:        true,
				},
			},
			{
				ID:         "PR_2",
				Number:     102,
				FromBranch: "spr/main/id-2",
				ToBranch:   "spr/main/id-1",
				Title:      "second",
				Commit:     commits[1],
				Commits:    []git.Commit{commits[0], commits[1]},
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPending,
					ReviewApproved: false,
					NoConflicts:    true,
					Stacked:        false,
				},
			},
		},
	}

	payload := BuildInfoPayload(cfg, info, commits)

	if payload.Repo.Owner != "owner" || payload.Repo.Name != "repo" {
		t.Errorf("repo = %+v", payload.Repo)
	}
	if payload.Repo.LocalBranch != "feature" {
		t.Errorf("localBranch = %q, want feature", payload.Repo.LocalBranch)
	}
	if !payload.Repo.RequireChecks || payload.Repo.RequireApproval {
		t.Errorf("config flags lost: %+v", payload.Repo)
	}
	if len(payload.Commits) != 3 || len(payload.PullRequests) != 2 {
		t.Fatalf("lengths: commits=%d prs=%d", len(payload.Commits), len(payload.PullRequests))
	}

	pr1 := payload.PullRequests[0]
	if pr1.MergeStatus.ChecksPass != "pass" {
		t.Errorf("pr1 checksPass = %q, want pass", pr1.MergeStatus.ChecksPass)
	}
	// Approval is not required, checks pass, no conflicts, stacked → mergeable.
	if !pr1.Mergeable {
		t.Error("pr1 should be mergeable")
	}
	if pr1.HasMultipleCommits {
		t.Error("pr1 should not be flagged as multi-commit")
	}

	pr2 := payload.PullRequests[1]
	if pr2.MergeStatus.ChecksPass != "pending" {
		t.Errorf("pr2 checksPass = %q, want pending", pr2.MergeStatus.ChecksPass)
	}
	if pr2.Mergeable {
		t.Error("pr2 must not be mergeable (pending checks, not stacked)")
	}
	if !pr2.HasMultipleCommits {
		t.Error("pr2 should be flagged as multi-commit")
	}
}

func TestBuildInfoPayloadEmpty(t *testing.T) {
	payload := BuildInfoPayload(testConfig(), nil, nil)
	if payload.PullRequests == nil || payload.Commits == nil {
		t.Error("slices must be non-nil for stable JSON ([] not null)")
	}
}

func TestCheckStatusString(t *testing.T) {
	tests := []struct {
		in   github.CheckStatus
		want string
	}{
		{github.CheckStatusUnknown, "unknown"},
		{github.CheckStatusPending, "pending"},
		{github.CheckStatusPass, "pass"},
		{github.CheckStatusFail, "fail"},
	}
	for _, tt := range tests {
		if got := checkStatusString(tt.in); got != tt.want {
			t.Errorf("checkStatusString(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
