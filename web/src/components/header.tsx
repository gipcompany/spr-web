import { GitPullRequestArrow, RefreshCw } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Spinner } from "@/components/ui/spinner"
import type { RepoInfo, RunStatus } from "@/lib/types"

interface HeaderProps {
  repo?: RepoInfo
  run: RunStatus
  busy: boolean
  refreshing: boolean
  disabled: boolean
  onUpdate: () => void
  onUpdateWithOptions: () => void
  onSync: () => void
  onCheck: () => void
  onMerge: () => void
  onRefresh: () => void
}

// Header carries every whole-stack action; per-PR actions live in the detail
// pane ("stack-wide = top, selected PR = right").
export function Header({
  repo,
  run,
  busy,
  refreshing,
  disabled,
  onUpdate,
  onUpdateWithOptions,
  onSync,
  onCheck,
  onMerge,
  onRefresh,
}: HeaderProps) {
  const actionsDisabled = busy || disabled
  return (
    <header className="flex items-center gap-3 border-b px-4 py-2">
      <GitPullRequestArrow className="size-5 shrink-0 text-muted-foreground" />
      <div className="min-w-0">
        <h1 className="truncate font-semibold text-sm">
          {repo ? `${repo.owner}/${repo.name}` : "spr-web"}
        </h1>
        {repo && (
          <p className="truncate text-muted-foreground text-xs">
            {repo.localBranch || "(detached)"} → {repo.remoteBranch}
          </p>
        )}
      </div>

      <div className="ml-auto flex items-center gap-2">
        {busy && (
          <span className="flex items-center gap-1.5 text-muted-foreground text-xs">
            <Spinner className="size-3.5" />
            {run.state === "running" ? `running ${run.cmd}` : "refreshing"}
          </span>
        )}
        <Button size="sm" disabled={actionsDisabled} onClick={onUpdate}>
          Update
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={actionsDisabled}
          onClick={onUpdateWithOptions}
        >
          Update…
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={actionsDisabled}
          onClick={onSync}
        >
          Sync
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={actionsDisabled}
          onClick={onCheck}
        >
          Check
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={actionsDisabled}
          onClick={onMerge}
        >
          Merge…
        </Button>
        <Button
          size="sm"
          variant="ghost"
          disabled={actionsDisabled}
          onClick={onRefresh}
          aria-label="Refresh stack"
        >
          <RefreshCw className={refreshing ? "animate-spin" : ""} />
        </Button>
      </div>
    </header>
  )
}
