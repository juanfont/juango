import { type ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from '../contexts/AuthContext'

interface ProtectedRouteProps {
  children: ReactNode
  loginPath?: string
  loadingComponent?: ReactNode
}

export function ProtectedRoute({
  children,
  loginPath = '/login',
  loadingComponent,
}: ProtectedRouteProps) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return loadingComponent ?? (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    )
  }

  if (!user) {
    return <Navigate to={loginPath} replace />
  }

  return <>{children}</>
}
