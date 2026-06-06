import * as React from "react"

import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import type { CommandRequest } from "@/lib/types"

interface UpdateOptionsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onRun: (body: CommandRequest) => void
}

// "Update with options" collects the update-only CLI flags: --reviewer,
// --no-rebase, --no-fetch. The plain Update button runs immediately without
// this dialog.
export function UpdateOptionsDialog({
  open,
  onOpenChange,
  onRun,
}: UpdateOptionsDialogProps) {
  const [reviewers, setReviewers] = React.useState("")
  const [noRebase, setNoRebase] = React.useState(false)
  const [noFetch, setNoFetch] = React.useState(false)

  const run = () => {
    const body: CommandRequest = { noRebase, noFetch }
    const list = reviewers
      .split(",")
      .map((r) => r.trim())
      .filter(Boolean)
    if (list.length > 0) body.reviewers = list
    onOpenChange(false)
    onRun(body)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Update with options</DialogTitle>
          <DialogDescription>
            Create and update pull requests for commits in the stack.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="reviewers">Reviewers</Label>
            <Input
              id="reviewers"
              placeholder="github-login, another-login"
              value={reviewers}
              onChange={(e) => setReviewers(e.target.value)}
            />
            <p className="text-muted-foreground text-xs">
              Added to newly created pull requests (comma separated).
            </p>
          </div>
          <Label className="flex items-center gap-2 font-normal">
            <Checkbox
              checked={noRebase}
              onCheckedChange={(v) => setNoRebase(v === true)}
            />
            Skip rebase (--no-rebase)
          </Label>
          <Label className="flex items-center gap-2 font-normal">
            <Checkbox
              checked={noFetch}
              onCheckedChange={(v) => setNoFetch(v === true)}
            />
            Skip fetch (--no-fetch)
          </Label>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={run}>Run update</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
