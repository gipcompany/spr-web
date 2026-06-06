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
} from "@/components/ui/alert-dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { predictMerge } from "@/lib/status"
import type { InfoPayload } from "@/lib/types"

interface MergeDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  info?: InfoPayload
  /** Prefill from "merge up to here"; undefined starts as "merge all". */
  initialCount?: number
  onConfirm: (count?: number) => void
}

// Merge is the one destructive stack action: confirm with the list of pull
// requests that are expected to merge. The count (--count N) is editable
// here; the list is a prediction from the cached info — spr makes the actual
// decision when the command runs.
export function MergeDialog({
  open,
  onOpenChange,
  info,
  initialCount,
  onConfirm,
}: MergeDialogProps) {
  const [countText, setCountText] = React.useState("")

  // Re-seed the input each time the dialog opens (prefilled by "merge up to
  // here", empty from the header Merge button).
  React.useEffect(() => {
    if (open) {
      setCountText(initialCount !== undefined ? String(initialCount) : "")
    }
  }, [open, initialCount])

  const trimmed = countText.trim()
  const valid = trimmed === "" || /^[1-9]\d*$/.test(trimmed)
  const count =
    valid && trimmed !== "" ? Number.parseInt(trimmed, 10) : undefined

  const predicted = info && valid ? predictMerge(info, count) : []
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {count !== undefined
              ? `Merge up to ${count} pull request${count === 1 ? "" : "s"}?`
              : "Merge all mergeable pull requests?"}
          </AlertDialogTitle>
          <AlertDialogDescription>
            spr merges mergeable pull requests from the bottom of the stack.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <div className="space-y-1.5">
          <Label htmlFor="merge-count">Count (--count)</Label>
          <Input
            id="merge-count"
            type="number"
            min={1}
            placeholder="all"
            aria-invalid={!valid}
            value={countText}
            onChange={(e) => setCountText(e.target.value)}
          />
          <p className="text-muted-foreground text-xs">
            Merge only the bottom N pull requests. Leave empty to merge
            everything mergeable.
          </p>
        </div>
        {!valid ? (
          <p className="text-red-600 text-sm dark:text-red-500">
            Count must be a whole number of at least 1.
          </p>
        ) : predicted.length === 0 ? (
          <p className="text-sm">
            No pull request currently looks mergeable. spr will re-evaluate when
            the command runs; nothing may happen.
          </p>
        ) : (
          <div className="text-sm">
            <p>The following pull requests are expected to merge:</p>
            <ul className="mt-2 list-disc space-y-1 pl-5 text-left">
              {predicted.map((pr) => (
                <li key={pr.id}>
                  <span className="font-mono">#{pr.number}</span> {pr.title}
                </li>
              ))}
            </ul>
            <p className="mt-2 text-muted-foreground text-xs">
              This list is a prediction based on the last fetched status; spr
              decides what actually merges at run time.
            </p>
          </div>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction disabled={!valid} onClick={() => onConfirm(count)}>
            Merge
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
