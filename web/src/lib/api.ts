import type {
  CommandName,
  CommandRequest,
  InfoResponse,
  RepoState,
  RunStatus,
  VersionInfo,
} from "./types"

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init)
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`
    try {
      const body = (await res.json()) as { error?: string }
      if (body.error) message = body.error
    } catch {
      // non-JSON error body; keep the status text
    }
    throw new ApiError(res.status, message)
  }
  if (res.status === 202) return undefined as T
  return (await res.json()) as T
}

export function getInfo(): Promise<InfoResponse> {
  return request("/api/info")
}

export function refreshInfo(): Promise<InfoResponse> {
  return request("/api/info/refresh", { method: "POST" })
}

export function getState(): Promise<RepoState> {
  return request("/api/state")
}

export function getRun(): Promise<RunStatus> {
  return request("/api/runs/current")
}

export function getVersion(): Promise<VersionInfo> {
  return request("/api/version")
}

export function postCommand(
  cmd: CommandName,
  body: CommandRequest,
): Promise<void> {
  return request(`/api/commands/${cmd}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  })
}

export function interruptRun(): Promise<void> {
  return request("/api/runs/current/interrupt", { method: "POST" })
}

export function runEventsUrl(): string {
  return "/api/runs/current/events"
}
