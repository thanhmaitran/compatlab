import { createContext, useContext, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import * as authService from '../../services/auth-service'
import { UNAUTHORIZED_EVENT, getToken } from '../../services/api-client'

interface AuthValue {
  authenticated: boolean
  signIn: (email: string, password: string) => Promise<void>
  signUp: (organization: string, email: string, password: string) => Promise<void>
  signOut: () => void
}

const AuthContext = createContext<AuthValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [authenticated, setAuthenticated] = useState(getToken() !== null)

  useEffect(() => {
    const onUnauthorized = () => setAuthenticated(false)
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
  }, [])

  const value = useMemo<AuthValue>(
    () => ({
      authenticated,
      signIn: async (email, password) => {
        await authService.login(email, password)
        setAuthenticated(true)
      },
      signUp: async (organization, email, password) => {
        await authService.register(organization, email, password)
        setAuthenticated(true)
      },
      signOut: () => {
        authService.logout()
        setAuthenticated(false)
      },
    }),
    [authenticated],
  )
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
