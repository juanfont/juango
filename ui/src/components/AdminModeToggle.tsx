import { useState } from "react"
import { Shield, Clock, Info, AlertCircle } from "lucide-react"
import { Button } from "./ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./ui/dialog"
import { Label } from "./ui/label"
import { Textarea } from "./ui/textarea"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "./ui/tooltip"
import { useAdmin } from "../contexts/AdminContext"
import { toast } from "sonner"

interface AdminModeToggleProps {
  timeoutMinutes?: number
}

export function AdminModeToggle({ timeoutMinutes = 30 }: AdminModeToggleProps) {
  const { isAdminMode, adminModeState, isLoading, enableAdminMode, disableAdminMode } = useAdmin()
  const [showDialog, setShowDialog] = useState(false)
  const [reason, setReason] = useState("")
  const [submitting, setSubmitting] = useState(false)

  const formatDuration = (since: string) => {
    const start = new Date(since)
    const now = new Date()
    const diff = now.getTime() - start.getTime()
    const minutes = Math.floor(diff / (1000 * 60))
    const hours = Math.floor(minutes / 60)

    if (hours > 0) {
      return `${hours}h ${minutes % 60}m`
    }
    return `${minutes}m`
  }

  const handleToggle = async () => {
    if (isAdminMode) {
      try {
        setSubmitting(true)
        await disableAdminMode()
        toast.success("Admin Mode Disabled", {
          description: "Administrative privileges have been revoked",
        })
      } catch (error) {
        toast.error("Failed to Disable Admin Mode", {
          description: error instanceof Error ? error.message : "An error occurred",
        })
      } finally {
        setSubmitting(false)
      }
    } else {
      setShowDialog(true)
    }
  }

  const handleEnable = async () => {
    if (!reason.trim()) {
      toast.error("Reason Required", {
        description: "Please provide a reason for enabling admin mode",
      })
      return
    }

    try {
      setSubmitting(true)
      await enableAdminMode(reason)
      setShowDialog(false)
      setReason("")
      toast.success("Admin Mode Enabled", {
        description: "You now have elevated administrative privileges",
      })
    } catch (error) {
      toast.error("Failed to Enable Admin Mode", {
        description: error instanceof Error ? error.message : "An error occurred",
      })
    } finally {
      setSubmitting(false)
    }
  }

  if (isLoading) {
    return null
  }

  return (
    <>
      <div className="flex items-center gap-3">
        <div className="flex flex-col items-end">
          <span className="text-sm font-medium">Admin Mode</span>
          {isAdminMode && adminModeState && (
            <div className="flex items-center gap-1 text-xs text-muted-foreground">
              <Clock className="h-3 w-3" />
              <span>{formatDuration(adminModeState.since)}</span>
            </div>
          )}
        </div>

        <button
          onClick={handleToggle}
          disabled={submitting}
          className={`
            relative inline-flex h-6 w-11 items-center rounded-full transition-colors
            focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2
            disabled:opacity-50 disabled:cursor-not-allowed
            ${isAdminMode ? 'bg-destructive' : 'bg-input'}
          `}
          aria-label={isAdminMode ? "Disable admin mode" : "Enable admin mode"}
        >
          <span
            className={`
              inline-block h-4 w-4 transform rounded-full bg-background transition-transform shadow-sm
              ${isAdminMode ? 'translate-x-6' : 'translate-x-1'}
            `}
          />
        </button>

        {isAdminMode && adminModeState && (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <div className="flex items-center gap-1 cursor-help">
                  <span className="text-xs font-medium text-destructive">ON</span>
                  <Info className="h-3 w-3 text-destructive" />
                </div>
              </TooltipTrigger>
              <TooltipContent className="max-w-xs">
                <div className="space-y-1">
                  <p className="font-medium">Admin Mode Active</p>
                  <p className="text-sm"><strong>Reason:</strong> {adminModeState.reason}</p>
                  <p className="text-sm"><strong>Since:</strong> {new Date(adminModeState.since).toLocaleString()}</p>
                  <p className="text-xs text-muted-foreground mt-2">
                    Will expire after {timeoutMinutes} minutes of inactivity
                  </p>
                </div>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        )}
      </div>

      <Dialog open={showDialog} onOpenChange={setShowDialog}>
        <DialogContent>
          <DialogHeader>
            <div className="flex items-center gap-2">
              <AlertCircle className="h-5 w-5 text-orange-500" />
              <DialogTitle>Enable Admin Mode</DialogTitle>
            </div>
            <DialogDescription>
              Admin mode grants elevated privileges for administrative tasks.
              Please provide a reason for audit purposes.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="reason">Reason for enabling admin mode</Label>
              <Textarea
                id="reason"
                placeholder="e.g., Investigating user access issue #1234"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                className="min-h-[100px]"
              />
            </div>

            <div className="rounded-md bg-yellow-50 dark:bg-yellow-900/20 p-3 text-sm">
              <p className="font-medium text-yellow-800 dark:text-yellow-200 flex items-center gap-2">
                <Shield className="h-4 w-4" />
                Important
              </p>
              <p className="mt-1 text-yellow-700 dark:text-yellow-300">
                All actions in admin mode are logged and subject to audit.
                Admin mode will automatically expire after {timeoutMinutes} minutes of inactivity.
              </p>
            </div>
          </div>

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setShowDialog(false)
                setReason("")
              }}
              disabled={submitting}
            >
              Cancel
            </Button>
            <Button
              onClick={handleEnable}
              disabled={submitting || !reason.trim()}
              className="bg-orange-600 hover:bg-orange-700"
            >
              {submitting ? "Enabling..." : "Enable Admin Mode"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
