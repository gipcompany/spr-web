import { AlertTriangle, X } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import type { RepoState } from "@/lib/types"

interface BannersProps {
  repoState?: RepoState
  connectionError: string | null
  commandError: string | null
  busy: boolean
  onEditDone: () => void
  onEditAbort: () => void
  onDismissCommandError: () => void
}

// Stacked page-level notices: edit session state machine, foreign rebase
// warning, connection failures and rejected commands.
export function Banners({
  repoState,
  connectionError,
  commandError,
  busy,
  onEditDone,
  onEditAbort,
  onDismissCommandError,
}: BannersProps) {
  const session = repoState?.editSession
  const foreignRebase = !session && repoState?.rebaseInProgress

  return (
    <div className="empty:hidden">
      {session && (
        <Alert className="rounded-none border-x-0 border-amber-300 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/40">
          <AlertTriangle className="text-amber-600 dark:text-amber-500" />
          <AlertTitle>
            {session.conflict
              ? "Edit session: resolving conflicts"
              : "Edit session in progress"}
          </AlertTitle>
          <AlertDescription>
            <span className="block">
              Editing commit: {session.subject || session.commitId}
            </span>
            <span className="block">
              {session.conflict
                ? "Resolve the conflicts in your local editor, stage the files, then click Done to continue the rebase."
                : "Make your changes in your local editor (files, git add), then click Done to amend and restore the stack."}
            </span>
            <span className="mt-2 flex gap-2">
              <Button size="sm" disabled={busy} onClick={onEditDone}>
                Done
              </Button>
              <Button
                size="sm"
                variant="outline"
                disabled={busy}
                onClick={onEditAbort}
              >
                Abort
              </Button>
            </span>
          </AlertDescription>
        </Alert>
      )}

      {foreignRebase && (
        <Alert variant="destructive" className="rounded-none border-x-0">
          <AlertTriangle />
          <AlertTitle>A git rebase is in progress outside spr-web</AlertTitle>
          <AlertDescription>
            The repository was left mid-rebase (possibly by an interrupted run).
            Resolve it in a terminal — for example with{" "}
            <code className="font-mono">git rebase --abort</code> — before
            running further commands.
          </AlertDescription>
        </Alert>
      )}

      {connectionError && (
        <Alert variant="destructive" className="rounded-none border-x-0">
          <AlertTriangle />
          <AlertTitle>Connection problem</AlertTitle>
          <AlertDescription>{connectionError}</AlertDescription>
        </Alert>
      )}

      {commandError && (
        <Alert className="rounded-none border-x-0">
          <AlertTriangle />
          <AlertTitle>Command rejected</AlertTitle>
          <AlertDescription>{commandError}</AlertDescription>
          <Button
            size="sm"
            variant="ghost"
            className="absolute top-2 right-2"
            onClick={onDismissCommandError}
            aria-label="Dismiss"
          >
            <X />
          </Button>
        </Alert>
      )}
    </div>
  )
}
