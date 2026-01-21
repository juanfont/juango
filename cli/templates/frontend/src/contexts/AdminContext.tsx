import { createContext, useContext, useState, useEffect, type ReactNode } from 'react'
import { getApiClient } from '../lib/api'
import type { AdminModeState } from '../lib/types'
import { useAuth } from './AuthContext'

interface AdminContextType {
  isAdminMode: boolean
  adminModeState: AdminModeState | null
  isLoading: boolean
  error: string | null
  enableAdminMode: (reason: string) => Promise<void>
  disableAdminMode: () => Promise<void>
  refreshStatus: () => Promise<void>
}

const AdminContext = createContext<AdminContextType | undefined>(undefined)

interface AdminProviderProps {
  children: ReactNode
}

export function AdminProvider({ children }: AdminProviderProps) {
  const { user } = useAuth()
  const [isAdminMode, setIsAdminMode] = useState(false)
  const [adminModeState, setAdminModeState] = useState<AdminModeState | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const apiClient = getApiClient()

  const refreshStatus = async () => {
    if (!user || !user.is_admin) {
      setIsAdminMode(false)
      setAdminModeState(null)
      setIsLoading(false)
      return
    }

    try {
      setIsLoading(true)
      setError(null)

      const status = await apiClient.getAdminModeStatus()

      if (status.admin_mode) {
        setIsAdminMode(status.admin_mode.enabled)
        setAdminModeState(status.admin_mode)
      } else {
        setIsAdminMode(false)
        setAdminModeState(null)
      }
    } catch (err) {
      console.error('Failed to get admin mode status:', err)
      setError('Failed to get admin mode status')
      setIsAdminMode(false)
      setAdminModeState(null)
    } finally {
      setIsLoading(false)
    }
  }

  const enableAdminMode = async (reason: string) => {
    try {
      setIsLoading(true)
      setError(null)

      const response = await apiClient.enableAdminMode(reason)

      if (response.state) {
        setIsAdminMode(true)
        setAdminModeState(response.state)
      }
    } catch (err) {
      console.error('Failed to enable admin mode:', err)
      setError('Failed to enable admin mode')
      throw err
    } finally {
      setIsLoading(false)
    }
  }

  const disableAdminMode = async () => {
    try {
      setIsLoading(true)
      setError(null)

      await apiClient.disableAdminMode()

      setIsAdminMode(false)
      setAdminModeState(null)
    } catch (err) {
      console.error('Failed to disable admin mode:', err)
      setError('Failed to disable admin mode')
      throw err
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    refreshStatus()
  }, [user])

  return (
    <AdminContext.Provider value={{
      isAdminMode,
      adminModeState,
      isLoading,
      error,
      enableAdminMode,
      disableAdminMode,
      refreshStatus
    }}>
      {children}
    </AdminContext.Provider>
  )
}

export function useAdmin() {
  const context = useContext(AdminContext)
  if (context === undefined) {
    throw new Error('useAdmin must be used within an AdminProvider')
  }
  return context
}
