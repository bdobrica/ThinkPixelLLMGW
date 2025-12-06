import { Routes, Route, Navigate } from 'react-router-dom'
import { Login } from './pages/Login'
import { Dashboard } from './pages/Dashboard'
import { ApiKeys } from './pages/ApiKeys'
import { Models } from './pages/Models'
import { Billing } from './pages/Billing'
import { ProtectedRoute } from './components/ProtectedRoute'
import { Layout } from './components/Layout'

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      
      <Route
        path="/"
        element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }
      >
        <Route index element={<Navigate to="/dashboard" replace />} />
        <Route path="dashboard" element={<Dashboard />} />
        <Route path="api-keys" element={<ApiKeys />} />
        <Route path="models" element={<Models />} />
        <Route path="billing" element={<Billing />} />
      </Route>
    </Routes>
  )
}
