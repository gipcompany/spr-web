import type { StackRow } from "@/hooks/use-app"
import { bitChar, bitClass, statusBits } from "@/lib/status"
import type { RepoInfo } from "@/lib/types"
import { cn } from "@/lib/utils"

interface StackListProps {
  rows: StackRow[]
  repo?: RepoInfo
  selectedCommitId?: string
  onSelect: (commitId: string) => void
}

// Left pane: the commit stack, top of stack first, with the same status bit
// string the spr CLI prints ([✔✗·✔]) rendered as colored spans.
export function StackList({
  rows,
  repo,
  selectedCommitId,
  onSelect,
}: StackListProps) {
  if (rows.length === 0) {
    return (
      <div className="flex h-full items-center justify-center p-6 text-center text-muted-foreground text-sm">
        The commit stack is empty.
      </div>
    )
  }
  return (
    <ul className="divide-y">
      {rows.map((row) => {
        const selected = row.commit.commitId === selectedCommitId
        return (
          <li key={row.commit.commitId}>
            <button
              type="button"
              onClick={() => onSelect(row.commit.commitId)}
              className={cn(
                "flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-muted/50",
                selected && "bg-muted",
              )}
            >
              <StatusBitsString row={row} repo={repo} />
              <span className="min-w-0 flex-1">
                <span className="block truncate">{row.commit.subject}</span>
                <span className="block truncate text-muted-foreground text-xs">
                  {row.pr ? (
                    <>
                      #{row.pr.number}
                      {row.pr.merged && " · merged"}
                      {row.pr.inQueue && " · in queue"}
                      {row.pr.hasMultipleCommits && " · ⚠ multiple commits"}
                    </>
                  ) : (
                    "no pull request yet"
                  )}
                  {row.commit.wip && " · WIP"}
                </span>
              </span>
            </button>
          </li>
        )
      })}
    </ul>
  )
}

function StatusBitsString({ row, repo }: { row: StackRow; repo?: RepoInfo }) {
  if (!row.pr || !repo) {
    return (
      <span className="shrink-0 font-mono text-muted-foreground text-xs">
        [ ]
      </span>
    )
  }
  return (
    <span className="shrink-0 font-mono text-xs">
      <span className="text-muted-foreground">[</span>
      {statusBits(row.pr, repo).map((bit) => (
        <span key={bit.key} className={bitClass(bit.state)} title={bit.detail}>
          {bitChar(bit.state)}
        </span>
      ))}
      <span className="text-muted-foreground">]</span>
    </span>
  )
}
