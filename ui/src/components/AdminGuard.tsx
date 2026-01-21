import { type ReactNode } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { useAdmin } from '../contexts/AdminContext'

interface AdminGuardProps {
  children: ReactNode
  requireAdminMode?: boolean
  fallback?: ReactNode
}

export function AdminGuard({
  children,
  requireAdminMode = false,
  fallback,
}: AdminGuardProps) {
  const { user } = useAuth()
  const { isAdminMode } = useAdmin()

  if (!user?.is_admin) {
    return fallback ?? null
  }

  if (requireAdminMode && !isAdminMode) {
    return fallback ?? null
  }

  return <>{children}</>
}
