import * as React from "react"

import { Banners } from "@/components/banners"
import { DetailPane } from "@/components/detail-pane"
import { Footer } from "@/components/footer"
import { Header } from "@/components/header"
import { LogPane } from "@/components/log-pane"
import { MergeDialog } from "@/components/merge-dialog"
import { StackList } from "@/components/stack-list"
import { UpdateOptionsDialog } from "@/components/update-options-dialog"
import { useApp } from "@/hooks/use-app"

// Fixed layout (pure CSS grid, no resizable panes): header / banners /
// two-pane main / log pane / footer.
export function App() {
  const app = useApp()
  const [mergeDialog, setMergeDialog] = React.useState<{
    open: boolean
    count?: number
  }>({ open: false })
  const [updateOptionsOpen, setUpdateOptionsOpen] = React.useState(false)

  const repo = app.info?.repo
  const editing = app.repoState?.editSession != null
  const foreignRebase = !editing && app.repoState?.rebaseInProgress === true
  // While the repo is mid-rebase only the edit-session buttons make sense.
  const actionsDisabled = editing || foreignRebase

  const countFromBottom = React.useMemo(() => {
    if (!app.info || !app.selected) return null
    const idx = app.info.commits.findIndex(
      (c) => c.commitId === app.selected?.commit.commitId,
    )
    return idx < 0 ? null : idx + 1
  }, [app.info, app.selected])

  return (
    <div className="grid h-svh grid-rows-[auto_auto_minmax(0,1fr)_14rem_auto]">
      <Header
        repo={repo}
        run={app.run}
        busy={app.busy}
        refreshing={app.refreshing}
        disabled={actionsDisabled}
        onUpdate={() => void app.runCommand("update")}
        onUpdateWithOptions={() => setUpdateOptionsOpen(true)}
        onSync={() => void app.runCommand("sync")}
        onCheck={() => void app.runCommand("check")}
        onMerge={() => setMergeDialog({ open: true })}
        onRefresh={() => void app.refresh()}
      />

      <Banners
        repoState={app.repoState}
        connectionError={app.connectionError}
        commandError={app.commandError}
        busy={app.busy}
        onEditDone={() => void app.runCommand("edit-done")}
        onEditAbort={() => void app.runCommand("edit-abort")}
        onDismissCommandError={app.dismissCommandError}
      />

      <main className="grid min-h-0 grid-cols-[24rem_minmax(0,1fr)]">
        <div className="min-h-0 overflow-y-auto border-r">
          <StackList
            rows={app.rows}
            repo={repo}
            selectedCommitId={app.selected?.commit.commitId}
            onSelect={app.select}
          />
        </div>
        <div className="min-h-0">
          <DetailPane
            row={app.selected}
            repo={repo}
            repoState={app.repoState}
            countFromBottom={countFromBottom}
            busy={app.busy}
            disabled={actionsDisabled}
            onAmend={(hash) => void app.runCommand("amend", { commit: hash })}
            onEdit={(hash) => void app.runCommand("edit", { commit: hash })}
            onUpdateUpToHere={(count) =>
              void app.runCommand("update", { count })
            }
            onMergeUpToHere={(count) => setMergeDialog({ open: true, count })}
          />
        </div>
      </main>

      <LogPane
        run={app.run}
        logs={app.logs}
        verbose={app.verbose}
        onVerboseChange={app.setVerbose}
        onInterrupt={() => void app.interrupt()}
      />

      <Footer version={app.version} fetchedAt={app.fetchedAt} />

      <MergeDialog
        open={mergeDialog.open}
        onOpenChange={(open) => setMergeDialog((prev) => ({ ...prev, open }))}
        info={app.info}
        initialCount={mergeDialog.count}
        onConfirm={(count) => {
          setMergeDialog({ open: false })
          void app.runCommand("merge", count !== undefined ? { count } : {})
        }}
      />

      <UpdateOptionsDialog
        open={updateOptionsOpen}
        onOpenChange={setUpdateOptionsOpen}
        onRun={(body) => void app.runCommand("update", body)}
      />
    </div>
  )
}

export default App
