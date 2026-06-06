import type { InfoPayload, PullRequestInfo, RepoInfo } from "./types"

// Status bit derivation, mirroring spr's CLI `[✔✗·✔]` output
// (github.PullRequest.StatusString): checks, review, conflicts, stacked.

export type BitState = "pass" | "fail" | "pending" | "unknown" | "empty"

export interface StatusBit {
  key: "checks" | "review" | "conflicts" | "stacked"
  label: string
  state: BitState
  detail: string
}

const BIT_CHARS: Record<BitState, string> = {
  pass: "✔",
  fail: "✗",
  pending: "·",
  unknown: "?",
  empty: "−",
}

const BIT_CLASSES: Record<BitState, string> = {
  pass: "text-green-600 dark:text-green-500",
  fail: "text-red-600 dark:text-red-500",
  pending: "text-amber-600 dark:text-amber-500",
  unknown: "text-muted-foreground",
  empty: "text-muted-foreground",
}

export function bitChar(state: BitState): string {
  return BIT_CHARS[state]
}

export function bitClass(state: BitState): string {
  return BIT_CLASSES[state]
}

export function statusBits(pr: PullRequestInfo, repo: RepoInfo): StatusBit[] {
  const checks: BitState = repo.requireChecks
    ? pr.mergeStatus.checksPass === "pass"
      ? "pass"
      : pr.mergeStatus.checksPass === "fail"
        ? "fail"
        : pr.mergeStatus.checksPass === "pending"
          ? "pending"
          : "unknown"
    : "empty"
  const review: BitState = repo.requireApproval
    ? pr.mergeStatus.reviewApproved
      ? "pass"
      : "fail"
    : "empty"
  const conflicts: BitState = pr.mergeStatus.noConflicts ? "pass" : "fail"
  const stacked: BitState = pr.mergeStatus.stacked ? "pass" : "fail"

  return [
    {
      key: "checks",
      label: "GitHub checks",
      state: checks,
      detail: repo.requireChecks
        ? `Checks: ${pr.mergeStatus.checksPass}`
        : "Checks are not required by this repository",
    },
    {
      key: "review",
      label: "Review approval",
      state: review,
      detail: repo.requireApproval
        ? pr.mergeStatus.reviewApproved
          ? "Approved"
          : "Not approved yet"
        : "Approval is not required by this repository",
    },
    {
      key: "conflicts",
      label: "No merge conflicts",
      state: conflicts,
      detail: pr.mergeStatus.noConflicts ? "No conflicts" : "Has conflicts",
    },
    {
      key: "stacked",
      label: "Stacked correctly",
      state: stacked,
      detail: pr.mergeStatus.stacked
        ? "All pull requests below are ready"
        : "A pull request below is not ready",
    },
  ]
}

export function prUrl(pr: PullRequestInfo, repo: RepoInfo): string {
  return `https://${repo.host}/${repo.owner}/${repo.name}/pull/${pr.number}`
}

// predictMerge lists the pull requests spr would merge: consecutive
// mergeable PRs from the bottom of the stack, optionally capped at count.
// This is a prediction based on the cached info; spr re-evaluates at run
// time.
export function predictMerge(
  info: InfoPayload,
  count?: number,
): PullRequestInfo[] {
  const result: PullRequestInfo[] = []
  for (const pr of info.pullRequests) {
    if (!pr.mergeable) break
    result.push(pr)
    if (count !== undefined && result.length >= count) break
  }
  return result
}
