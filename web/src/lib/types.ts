// API contract types. These mirror the Go DTOs in internal/sprbridge (info
// payload) and internal/server (state, run, version).

export interface RepoInfo {
  owner: string
  name: string
  host: string
  remoteBranch: string
  localBranch: string
  requireChecks: boolean
  requireApproval: boolean
  mergeQueue: boolean
  mergeCheck?: string
}

export interface CommitInfo {
  commitId: string
  commitHash: string
  subject: string
  body?: string
  wip: boolean
}

export type CheckState = "unknown" | "pending" | "pass" | "fail"

export interface MergeStatusInfo {
  checksPass: CheckState
  reviewApproved: boolean
  noConflicts: boolean
  stacked: boolean
}

export interface PullRequestInfo {
  id: string
  number: number
  fromBranch: string
  toBranch: string
  title: string
  commit: CommitInfo
  localCommitHash?: string
  merged: boolean
  inQueue: boolean
  hasMultipleCommits: boolean
  mergeStatus: MergeStatusInfo
  mergeable: boolean
}

export interface InfoPayload {
  repo: RepoInfo
  pullRequests: PullRequestInfo[]
  // Local commit stack, bottom (oldest) first.
  commits: CommitInfo[]
}

export interface InfoResponse {
  info: InfoPayload
  fetchedAt: string
}

export interface WorkingTree {
  staged: number
  unstaged: number
  untracked: number
}

export interface EditSession {
  commitId: string
  subject: string
  conflict: boolean
}

export interface RepoState {
  editSession: EditSession | null
  rebaseInProgress: boolean
  workingTree: WorkingTree
}

export type RunStatus =
  | { state: "idle" }
  | { state: "running"; cmd: CommandName; startedAt: string }
  | {
      state: "exited"
      cmd: CommandName
      startedAt: string
      exitCode: number
      signal?: string
    }

export type LogStream = "stdout" | "stderr" | "server"

export interface LogLine {
  stream: LogStream
  text: string
}

export interface VersionInfo {
  version: string
  commit: string
  date: string
  spr: string
}

export type CommandName =
  | "update"
  | "sync"
  | "check"
  | "merge"
  | "amend"
  | "edit"
  | "edit-done"
  | "edit-abort"

export interface CommandRequest {
  commit?: string
  count?: number
  reviewers?: string[]
  noRebase?: boolean
  noFetch?: boolean
  verbose?: boolean
}

// Contract exit codes produced by the spr-web child wrapper (see
// internal/sprbridge/exitcodes.go). spr's own exit codes are not a contract.
export const EXIT_COMMIT_NOT_FOUND = 10
export const EXIT_NO_STAGED = 11
