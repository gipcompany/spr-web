import { ExternalLink } from "lucide-react"

import { Button } from "@/components/ui/button"
import type { StackRow } from "@/hooks/use-app"
import { bitChar, bitClass, prUrl, statusBits } from "@/lib/status"
import type { RepoInfo, RepoState } from "@/lib/types"
import { cn } from "@/lib/utils"

interface DetailPaneProps {
  row: StackRow | null
  repo?: RepoInfo
  repoState?: RepoState
  /** 1-based position from the bottom of the stack (== --count value). */
  countFromBottom: number | null
  busy: boolean
  disabled: boolean
  onAmend: (commitHash: string) => void
  onEdit: (commitHash: string) => void
  onUpdateUpToHere: (count: number) => void
  onMergeUpToHere: (count: number) => void
}

// Right pane: details and actions for the selected commit / pull request.
// All per-PR actions live here (no row menus in the list).
export function DetailPane({
  row,
  repo,
  repoState,
  countFromBottom,
  busy,
  disabled,
  onAmend,
  onEdit,
  onUpdateUpToHere,
  onMergeUpToHere,
}: DetailPaneProps) {
  if (!row) {
    return (
      <div className="flex h-full items-center justify-center p-6 text-muted-foreground text-sm">
        Select a commit from the stack.
      </div>
    )
  }

  const { commit, pr } = row
  const staged = repoState?.workingTree.staged ?? 0
  const actionsDisabled = busy || disabled
  const amendDisabled = actionsDisabled || staged === 0

  return (
    <div className="flex h-full flex-col gap-4 overflow-y-auto p-4">
      <section>
        <h2 className="font-medium text-base">{commit.subject}</h2>
        <p className="mt-1 font-mono text-muted-foreground text-xs">
          {commit.commitHash.slice(0, 12)} · commit-id {commit.commitId}
          {commit.wip && " · WIP"}
        </p>
        {commit.body && (
          <pre className="mt-2 whitespace-pre-wrap font-sans text-muted-foreground text-sm">
            {commit.body}
          </pre>
        )}
      </section>

      {pr && repo ? (
        <section className="rounded-lg border p-3">
          <div className="flex items-center justify-between gap-2">
            <h3 className="font-medium text-sm">
              Pull request #{pr.number}
              {pr.merged && " (merged)"}
              {pr.inQueue && " (in merge queue)"}
            </h3>
            <a
              href={prUrl(pr, repo)}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1 text-blue-600 text-xs hover:underline dark:text-blue-500"
            >
              Open on GitHub <ExternalLink className="size-3" />
            </a>
          </div>
          <p className="mt-0.5 truncate text-muted-foreground text-xs">
            {pr.fromBranch} → {pr.toBranch}
          </p>
          <dl className="mt-2 space-y-1 text-sm">
            {statusBits(pr, repo).map((bit) => (
              <div key={bit.key} className="flex items-center gap-2">
                <dt className="w-36 text-muted-foreground">{bit.label}</dt>
                <dd className={cn("font-mono", bitClass(bit.state))}>
                  {bitChar(bit.state)}
                  <span className="ml-2 font-sans text-foreground text-xs">
                    {bit.detail}
                  </span>
                </dd>
              </div>
            ))}
            <div className="flex items-center gap-2 border-t pt-1">
              <dt className="w-36 text-muted-foreground">Mergeable</dt>
              <dd
                className={cn(
                  "font-medium",
                  pr.mergeable
                    ? "text-green-600 dark:text-green-500"
                    : "text-muted-foreground",
                )}
              >
                {pr.mergeable ? "yes" : "not yet"}
              </dd>
            </div>
          </dl>
          {pr.hasMultipleCommits && (
            <p className="mt-2 text-amber-600 text-xs dark:text-amber-500">
              ⚠ This pull request contains more than one commit.
            </p>
          )}
        </section>
      ) : (
        <section className="rounded-lg border border-dashed p-3 text-muted-foreground text-sm">
          No pull request for this commit yet — run Update to create the missing
          pull requests.
        </section>
      )}

      <section className="mt-auto space-y-2 border-t pt-3">
        <h3 className="font-medium text-muted-foreground text-xs uppercase tracking-wide">
          Actions for this commit
        </h3>
        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            variant="outline"
            disabled={amendDisabled}
            onClick={() => onAmend(commit.commitHash)}
          >
            Amend with staged changes
          </Button>
          <Button
            size="sm"
            variant="outline"
            disabled={actionsDisabled}
            onClick={() => onEdit(commit.commitHash)}
          >
            Edit (interactive rebase)
          </Button>
          {countFromBottom !== null && (
            <>
              <Button
                size="sm"
                variant="outline"
                disabled={actionsDisabled}
                onClick={() => onUpdateUpToHere(countFromBottom)}
              >
                Update up to here ({countFromBottom})
              </Button>
              <Button
                size="sm"
                variant="outline"
                disabled={actionsDisabled || !pr}
                onClick={() => onMergeUpToHere(countFromBottom)}
              >
                Merge up to here… ({countFromBottom})
              </Button>
            </>
          )}
        </div>
        {staged === 0 && !busy && (
          <p className="text-muted-foreground text-xs">
            Amend is unavailable: no staged changes in the working tree.
          </p>
        )}
      </section>
    </div>
  )
}
