import { useState } from "react"
import { useAuth } from "../contexts/AuthContext"
import { getApiClient } from "../lib/api"
import { Button } from "./ui/button"
import { AlertTriangle, X } from "lucide-react"
import { toast } from "sonner"

export function ImpersonationBanner() {
  const { session } = useAuth()
  const [loading, setLoading] = useState(false)
  const apiClient = getApiClient()

  if (!session?.impersonation || !session.impersonation.enabled) {
    return null
  }

  const handleStopImpersonation = async () => {
    try {
      setLoading(true)
      await apiClient.stopImpersonation()
      toast.success("Impersonation stopped")
      window.location.reload()
    } catch (error: unknown) {
      console.error("Failed to stop impersonation:", error)
      const errorMessage = error instanceof Error ? error.message : "Failed to stop impersonation"
      toast.error(errorMessage)
      setLoading(false)
    }
  }

  return (
    <div className="bg-yellow-500 text-yellow-950 px-4 py-3 shadow-md">
      <div className="max-w-7xl mx-auto flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <AlertTriangle className="h-5 w-5 flex-shrink-0" />
          <div className="flex-1">
            <p className="font-semibold text-sm">
              You are impersonating {session.impersonation.target_user_name} (
              {session.impersonation.target_user_email})
            </p>
            <p className="text-xs mt-0.5">
              Reason: {session.impersonation.reason} - All actions are being logged
            </p>
          </div>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={handleStopImpersonation}
          disabled={loading}
          className="bg-yellow-950 text-yellow-50 hover:bg-yellow-900 border-yellow-800 flex-shrink-0"
        >
          <X className="h-4 w-4 mr-1" />
          Exit Impersonation
        </Button>
      </div>
    </div>
  )
}
