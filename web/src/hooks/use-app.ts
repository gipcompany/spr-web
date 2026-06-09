import * as React from "react"

import * as api from "@/lib/api"
import type {
  CommandName,
  CommandRequest,
  CommitInfo,
  InfoPayload,
  LogLine,
  PullRequestInfo,
  RepoState,
  RunStatus,
  VersionInfo,
} from "@/lib/types"
import { EXIT_COMMIT_NOT_FOUND, EXIT_NO_STAGED } from "@/lib/types"

const MAX_CLIENT_LOG_LINES = 5000

export interface StackRow {
  commit: CommitInfo
  pr?: PullRequestInfo
}

export interface AppState {
  info?: InfoPayload
  fetchedAt?: string
  repoState?: RepoState
  run: RunStatus
  logs: LogLine[]
  version?: VersionInfo
  /** Connection-level failure (server unreachable, stream lost). */
  connectionError: string | null
  /** Rejected command / refresh (409, 422, ...) or contract-exit notice. */
  commandError: string | null
  refreshing: boolean
  verbose: boolean
  busy: boolean
  /** Stack rows, top of stack first (same order as `git spr status`). */
  rows: StackRow[]
  selected: StackRow | null
}

export interface AppActions {
  refresh: () => Promise<void>
  runCommand: (cmd: CommandName, body?: CommandRequest) => Promise<boolean>
  interrupt: () => Promise<void>
  select: (commitId: string) => void
  setVerbose: (v: boolean) => void
  dismissCommandError: () => void
}

export function useApp(): AppState & AppActions {
  const [info, setInfo] = React.useState<InfoPayload>()
  const [fetchedAt, setFetchedAt] = React.useState<string>()
  const [repoState, setRepoState] = React.useState<RepoState>()
  const [run, setRun] = React.useState<RunStatus>({ state: "idle" })
  const [logs, setLogs] = React.useState<LogLine[]>([])
  const [version, setVersion] = React.useState<VersionInfo>()
  const [connectionError, setConnectionError] = React.useState<string | null>(
    null,
  )
  const [commandError, setCommandError] = React.useState<string | null>(null)
  const [refreshing, setRefreshing] = React.useState(false)
  const [verbose, setVerbose] = React.useState(false)
  const [selectedId, setSelectedId] = React.useState<string | null>(null)

  const esRef = React.useRef<EventSource | null>(null)
  const exitedRef = React.useRef(false)

  const fetchInfo = React.useCallback(async () => {
    const r = await api.getInfo()
    setInfo(r.info)
    setFetchedAt(r.fetchedAt)
  }, [])

  const fetchState = React.useCallback(async () => {
    setRepoState(await api.getState())
  }, [])

  // attach subscribes to the run stream. The server replays the buffered log
  // from the start, then tails; the stream closes after `exit` (and the
  // post-run `info-updated` when applicable).
  const attach = React.useCallback(() => {
    esRef.current?.close()
    exitedRef.current = false

    const es = new EventSource(api.runEventsUrl())
    esRef.current = es

    es.onopen = () => {
      // Full replay follows: drop whatever we had.
      setLogs([])
      setConnectionError(null)
    }
    es.addEventListener("log", (e) => {
      const line = JSON.parse((e as MessageEvent).data) as LogLine
      setLogs((prev) =>
        prev.length >= MAX_CLIENT_LOG_LINES
          ? [...prev.slice(-(MAX_CLIENT_LOG_LINES - 1)), line]
          : [...prev, line],
      )
    })
    es.addEventListener("exit", (e) => {
      exitedRef.current = true
      const { code } = JSON.parse((e as MessageEvent).data) as { code: number }
      if (code === EXIT_COMMIT_NOT_FOUND) {
        setCommandError(
          "The commit stack changed before the command could run. Refresh and try again.",
        )
      } else if (code === EXIT_NO_STAGED) {
        setCommandError("No staged changes to amend.")
      }
      void api.getRun().then(setRun).catch(noop)
      void fetchState().catch(noop)
    })
    es.addEventListener("info-updated", () => {
      void fetchInfo().catch(noop)
    })
    es.onerror = () => {
      if (exitedRef.current) {
        // Normal end of stream: the server closes after the run finishes.
        es.close()
        if (esRef.current === es) esRef.current = null
      } else {
        setConnectionError("Lost connection to the run stream; reconnecting…")
      }
    }
  }, [fetchInfo, fetchState])

  // Initial load: cached info, repo state, version, and the current run.
  // When a run is in flight (or just finished) we attach to replay its log —
  // a page reload never loses a running command.
  React.useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        await fetchInfo()
        await fetchState()
        setVersion(await api.getVersion())
        const current = await api.getRun()
        if (cancelled) return
        setRun(current)
        if (current.state !== "idle") attach()
      } catch {
        if (!cancelled) {
          setConnectionError("Cannot reach the spr-web server.")
        }
      }
    })()
    return () => {
      cancelled = true
      esRef.current?.close()
      esRef.current = null
    }
  }, [attach, fetchInfo, fetchState])

  const runCommand = React.useCallback(
    async (cmd: CommandName, body: CommandRequest = {}) => {
      setCommandError(null)
      try {
        await api.postCommand(cmd, { ...body, verbose })
      } catch (err) {
        setCommandError(err instanceof Error ? err.message : String(err))
        // A guard rejection usually means the client state is stale.
        void fetchState().catch(noop)
        return false
      }
      setRun({ state: "running", cmd, startedAt: new Date().toISOString() })
      setLogs([])
      attach()
      void fetchState().catch(noop)
      return true
    },
    [verbose, attach, fetchState],
  )

  const refresh = React.useCallback(async () => {
    setCommandError(null)
    setRefreshing(true)
    try {
      const r = await api.refreshInfo()
      setInfo(r.info)
      setFetchedAt(r.fetchedAt)
      await fetchState()
    } catch (err) {
      setCommandError(err instanceof Error ? err.message : String(err))
    } finally {
      setRefreshing(false)
    }
  }, [fetchState])

  const interrupt = React.useCallback(async () => {
    try {
      await api.interruptRun()
    } catch (err) {
      setCommandError(err instanceof Error ? err.message : String(err))
    }
  }, [])

  const rows = React.useMemo<StackRow[]>(() => {
    if (!info) return []
    const byCommitId = new Map(
      info.pullRequests.map((pr) => [pr.commit.commitId, pr]),
    )
    // Top of stack first, like `git spr status`.
    return [...info.commits]
      .reverse()
      .map((commit) => ({ commit, pr: byCommitId.get(commit.commitId) }))
  }, [info])

  const selected = React.useMemo(() => {
    if (rows.length === 0) return null
    const found = rows.find((r) => r.commit.commitId === selectedId)
    if (found) return found
    // Focus was never set, or its commit left the stack (typically a pull
    // request that merged and dropped out of the local stack on update). rows
    // is top-of-stack first, so the oldest row is last: prefer the oldest pull
    // request still open — that is the next one to act on in a bottom-up merge
    // flow — then degrade to the oldest commit so the detail pane is never
    // blank while the stack is non-empty.
    for (let i = rows.length - 1; i >= 0; i--) {
      const { pr } = rows[i]
      if (pr && !pr.merged) return rows[i]
    }
    return rows[rows.length - 1]
  }, [rows, selectedId])

  const busy = run.state === "running" || refreshing

  return {
    info,
    fetchedAt,
    repoState,
    run,
    logs,
    version,
    connectionError,
    commandError,
    refreshing,
    verbose,
    busy,
    rows,
    selected,
    refresh,
    runCommand,
    interrupt,
    select: setSelectedId,
    setVerbose,
    dismissCommandError: () => setCommandError(null),
  }
}

function noop() {}
