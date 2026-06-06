package sprbridge

import (
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
)

// InfoPayload is the JSON contract between the `_spr info` child process and
// the server (and, through the server cache, the web UI). It carries only
// data spr itself exposes: github.GetInfo() + git.GetLocalCommitStack() plus
// the few repo config flags needed to interpret the status bits.
type InfoPayload struct {
	Repo         RepoInfo          `json:"repo"`
	PullRequests []PullRequestInfo `json:"pullRequests"`
	// Commits is the local commit stack, bottom (oldest) first.
	Commits []CommitInfo `json:"commits"`
}

// RepoInfo describes the repository and the config flags that affect how
// merge status bits should be rendered.
type RepoInfo struct {
	Owner           string `json:"owner"`
	Name            string `json:"name"`
	Host            string `json:"host"`
	RemoteBranch    string `json:"remoteBranch"`
	LocalBranch     string `json:"localBranch"`
	RequireChecks   bool   `json:"requireChecks"`
	RequireApproval bool   `json:"requireApproval"`
	MergeQueue      bool   `json:"mergeQueue"`
	MergeCheck      string `json:"mergeCheck,omitempty"`
}

// CommitInfo mirrors git.Commit.
type CommitInfo struct {
	CommitID   string `json:"commitId"`
	CommitHash string `json:"commitHash"`
	Subject    string `json:"subject"`
	Body       string `json:"body,omitempty"`
	WIP        bool   `json:"wip"`
}

// PullRequestInfo mirrors github.PullRequest with the merge status bits
// flattened into JSON-friendly values.
type PullRequestInfo struct {
	ID                 string          `json:"id"`
	Number             int             `json:"number"`
	FromBranch         string          `json:"fromBranch"`
	ToBranch           string          `json:"toBranch"`
	Title              string          `json:"title"`
	Commit             CommitInfo      `json:"commit"`
	LocalCommitHash    string          `json:"localCommitHash,omitempty"`
	Merged             bool            `json:"merged"`
	InQueue            bool            `json:"inQueue"`
	HasMultipleCommits bool            `json:"hasMultipleCommits"`
	MergeStatus        MergeStatusInfo `json:"mergeStatus"`
	// Mergeable is spr's own pr.Mergeable(config) verdict computed at fetch
	// time; the actual decision is re-made by spr when merge runs.
	Mergeable bool `json:"mergeable"`
}

// MergeStatusInfo mirrors github.PullRequestMergeStatus.
type MergeStatusInfo struct {
	// ChecksPass is one of "unknown", "pending", "pass", "fail".
	ChecksPass     string `json:"checksPass"`
	ReviewApproved bool   `json:"reviewApproved"`
	NoConflicts    bool   `json:"noConflicts"`
	Stacked        bool   `json:"stacked"`
}

func checkStatusString(cs github.CheckStatus) string {
	switch cs {
	case github.CheckStatusPending:
		return "pending"
	case github.CheckStatusPass:
		return "pass"
	case github.CheckStatusFail:
		return "fail"
	default:
		return "unknown"
	}
}

func commitInfo(c git.Commit) CommitInfo {
	return CommitInfo{
		CommitID:   c.CommitID,
		CommitHash: c.CommitHash,
		Subject:    c.Subject,
		Body:       c.Body,
		WIP:        c.WIP,
	}
}

// BuildInfoPayload converts spr's structured data into the API payload.
func BuildInfoPayload(cfg *config.Config, info *github.GitHubInfo, commits []git.Commit) InfoPayload {
	payload := InfoPayload{
		Repo: RepoInfo{
			Owner:           cfg.Repo.GitHubRepoOwner,
			Name:            cfg.Repo.GitHubRepoName,
			Host:            cfg.Repo.GitHubHost,
			RemoteBranch:    cfg.Repo.GitHubBranch,
			RequireChecks:   cfg.Repo.RequireChecks,
			RequireApproval: cfg.Repo.RequireApproval,
			MergeQueue:      cfg.Repo.MergeQueue,
			MergeCheck:      cfg.Repo.MergeCheck,
		},
		PullRequests: []PullRequestInfo{},
		Commits:      []CommitInfo{},
	}
	if info != nil {
		payload.Repo.LocalBranch = info.LocalBranch
		for _, pr := range info.PullRequests {
			if pr == nil {
				continue
			}
			payload.PullRequests = append(payload.PullRequests, PullRequestInfo{
				ID:                 pr.ID,
				Number:             pr.Number,
				FromBranch:         pr.FromBranch,
				ToBranch:           pr.ToBranch,
				Title:              pr.Title,
				Commit:             commitInfo(pr.Commit),
				LocalCommitHash:    pr.LocalCommitHash,
				Merged:             pr.Merged,
				InQueue:            pr.InQueue,
				HasMultipleCommits: len(pr.Commits) > 1,
				MergeStatus: MergeStatusInfo{
					ChecksPass:     checkStatusString(pr.MergeStatus.ChecksPass),
					ReviewApproved: pr.MergeStatus.ReviewApproved,
					NoConflicts:    pr.MergeStatus.NoConflicts,
					Stacked:        pr.MergeStatus.Stacked,
				},
				Mergeable: pr.Mergeable(cfg),
			})
		}
	}
	for _, c := range commits {
		payload.Commits = append(payload.Commits, commitInfo(c))
	}
	return payload
}
