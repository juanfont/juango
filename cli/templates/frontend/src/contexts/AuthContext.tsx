import { createContext, useContext, useEffect, useState, useCallback, type ReactNode } from 'react'
import { getApiClient } from '../lib/api'
import type { User, SessionResponse } from '../lib/types'

interface AuthContextType {
  user: User | null
  session: SessionResponse | null
  isLoading: boolean
  login: () => void
  logout: () => Promise<void>
  refreshAuth: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

interface AuthProviderProps {
  children: ReactNode
  loginPath?: string
}

export function AuthProvider({ children, loginPath = '/login' }: AuthProviderProps) {
  const [user, setUser] = useState<User | null>(null)
  const [session, setSession] = useState<SessionResponse | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const apiClient = getApiClient()

  const clearAuth = useCallback(() => {
    setUser(null)
    setSession(null)
  }, [])

  const checkAuth = useCallback(async (showLoadingState = true) => {
    if (window.location.pathname === loginPath) {
      setIsLoading(false)
      return
    }

    if (showLoadingState) {
      setIsLoading(true)
    }

    try {
      const data: SessionResponse = await apiClient.checkSession()

      if (data.authenticated && data.user) {
        setUser(data.user)
        setSession(data)
      } else {
        clearAuth()
        if (window.location.pathname !== loginPath) {
          window.location.href = loginPath
        }
      }
    } catch (error) {
      console.error('Auth check failed:', error)
      clearAuth()
      if (window.location.pathname !== loginPath) {
        window.location.href = loginPath
      }
    } finally {
      setIsLoading(false)
    }
  }, [clearAuth, apiClient, loginPath])

  const refreshAuth = useCallback(async () => {
    await checkAuth(false)
  }, [checkAuth])

  useEffect(() => {
    checkAuth()
  }, [checkAuth])

  const login = () => {
    apiClient.login()
  }

  const logout = async () => {
    try {
      await apiClient.logout()
      clearAuth()
    } catch (error) {
      console.error('Logout failed:', error)
      clearAuth()
      window.location.href = loginPath
    }
  }

  return (
    <AuthContext.Provider value={{ user, session, isLoading, login, logout, refreshAuth }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
