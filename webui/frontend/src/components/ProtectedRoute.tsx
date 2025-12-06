import { ReactNode, useEffect, useState } from 'react'
import { Navigate } from 'react-router-dom'
import { authAPI } from '../api/client'

interface ProtectedRouteProps {
  children: ReactNode
}

export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const [isAuthenticated, setIsAuthenticated] = useState<boolean | null>(null)

  useEffect(() => {
    authAPI
      .me()
      .then(() => setIsAuthenticated(true))
      .catch(() => setIsAuthenticated(false))
  }, [])

  // Still checking auth
  if (isAuthenticated === null) {
    return (
      <main className="container">
        <p aria-busy="true">Loading...</p>
      </main>
    )
  }

  // Not authenticated, redirect to login
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  // Authenticated, render children
  return <>{children}</>
}
