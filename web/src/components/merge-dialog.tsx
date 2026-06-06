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
import { predictMerge } from "@/lib/status"
import type { InfoPayload } from "@/lib/types"

interface MergeDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  info?: InfoPayload
  /** Cap from "merge up to here"; undefined merges everything mergeable. */
  count?: number
  onConfirm: (count?: number) => void
}

// Merge is the one destructive stack action: confirm with the list of pull
// requests that are expected to merge. The list is a prediction from the
// cached info — spr makes the actual decision when the command runs.
export function MergeDialog({
  open,
  onOpenChange,
  info,
  count,
  onConfirm,
}: MergeDialogProps) {
  const predicted = info ? predictMerge(info, count) : []
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {count !== undefined
              ? `Merge up to ${count} pull request${count === 1 ? "" : "s"}?`
              : "Merge all mergeable pull requests?"}
          </AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div>
              {predicted.length === 0 ? (
                <p>
                  No pull request currently looks mergeable. spr will
                  re-evaluate when the command runs; nothing may happen.
                </p>
              ) : (
                <>
                  <p>The following pull requests are expected to merge:</p>
                  <ul className="mt-2 list-disc space-y-1 pl-5 text-left">
                    {predicted.map((pr) => (
                      <li key={pr.id} className="text-sm">
                        <span className="font-mono">#{pr.number}</span>{" "}
                        {pr.title}
                      </li>
                    ))}
                  </ul>
                  <p className="mt-2 text-muted-foreground text-xs">
                    This list is a prediction based on the last fetched status;
                    spr decides what actually merges at run time.
                  </p>
                </>
              )}
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction onClick={() => onConfirm(count)}>
            Merge
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
