import * as React from "react"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import { Spinner } from "@/components/ui/spinner"
import { parseAnsi } from "@/lib/ansi"
import type { LogLine, RunStatus } from "@/lib/types"
import { cn } from "@/lib/utils"

interface LogPaneProps {
  run: RunStatus
  logs: LogLine[]
  verbose: boolean
  onVerboseChange: (v: boolean) => void
  onInterrupt: () => void
}

// Bottom pane: the run log (SSE replay + tail). stdout is command output,
// stderr carries verbose/diagnostic logs.
export function LogPane({
  run,
  logs,
  verbose,
  onVerboseChange,
  onInterrupt,
}: LogPaneProps) {
  const bodyRef = React.useRef<HTMLDivElement>(null)
  const pinnedRef = React.useRef(true)

  // Auto-follow: keep scrolled to the bottom unless the user scrolled up.
  // biome-ignore lint/correctness/useExhaustiveDependencies: scrolling reacts to new log lines
  React.useEffect(() => {
    const el = bodyRef.current
    if (el && pinnedRef.current) {
      el.scrollTop = el.scrollHeight
    }
  }, [logs])

  const onScroll = () => {
    const el = bodyRef.current
    if (!el) return
    pinnedRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24
  }

  return (
    <div className="flex h-full min-h-0 flex-col border-t">
      <div className="flex items-center gap-3 border-b bg-muted/30 px-3 py-1.5">
        <span className="font-medium text-muted-foreground text-xs uppercase tracking-wide">
          Run log
        </span>
        <RunStatusBadge run={run} />
        <div className="ml-auto flex items-center gap-3">
          <Label className="flex items-center gap-1.5 font-normal text-xs">
            <Checkbox
              checked={verbose}
              onCheckedChange={(v) => onVerboseChange(v === true)}
            />
            Verbose logs
          </Label>
          {run.state === "running" && (
            <InterruptButton onInterrupt={onInterrupt} />
          )}
        </div>
      </div>
      <div
        ref={bodyRef}
        onScroll={onScroll}
        className="min-h-0 flex-1 overflow-y-auto bg-background px-3 py-2 font-mono text-xs leading-5"
      >
        {logs.length === 0 ? (
          <p className="text-muted-foreground">
            {run.state === "idle"
              ? "No command has run yet."
              : run.state === "running"
                ? "Waiting for output…"
                : "The last run produced no output."}
          </p>
        ) : (
          logs.map((line, i) => (
            <div
              // biome-ignore lint/suspicious/noArrayIndexKey: append-only log lines
              key={i}
              className={cn(
                "whitespace-pre-wrap break-all",
                line.stream === "stderr" && "text-muted-foreground",
                line.stream === "server" &&
                  "text-amber-700 italic dark:text-amber-500",
              )}
            >
              {parseAnsi(line.text).map((seg, j) => (
                // biome-ignore lint/suspicious/noArrayIndexKey: static segments
                <span key={j} className={seg.className}>
                  {seg.text}
                </span>
              ))}
              {line.text === "" ? " " : null}
            </div>
          ))
        )}
      </div>
    </div>
  )
}

function RunStatusBadge({ run }: { run: RunStatus }) {
  if (run.state === "idle") return null
  if (run.state === "running") {
    return (
      <span className="flex items-center gap-1.5 text-muted-foreground text-xs">
        <Spinner className="size-3" />
        {run.cmd}
      </span>
    )
  }
  const ok = run.exitCode === 0
  return (
    <span
      className={cn(
        "text-xs",
        ok
          ? "text-green-600 dark:text-green-500"
          : "text-red-600 dark:text-red-500",
      )}
    >
      {run.cmd} — {ok ? "succeeded" : failureText(run.exitCode, run.signal)}
    </span>
  )
}

function failureText(code: number, signal?: string): string {
  if (signal) return `terminated (${signal})`
  return `failed with exit code ${code}`
}

// Interrupting kills a child mid-git-operation; SIGINT lets git clean up its
// lock files, then SIGKILL after a grace period. The confirmation warns that
// the repository may be left mid-rebase.
function InterruptButton({ onInterrupt }: { onInterrupt: () => void }) {
  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>
        <Button size="sm" variant="destructive">
          Interrupt
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Interrupt the running command?</AlertDialogTitle>
          <AlertDialogDescription>
            The command is killed mid-flight (SIGINT, then SIGKILL after 5
            seconds). If a rebase was in progress the repository may be left in
            a mid-rebase state; spr-web will then show a warning with recovery
            instructions (git rebase --abort).
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction onClick={onInterrupt}>Interrupt</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
