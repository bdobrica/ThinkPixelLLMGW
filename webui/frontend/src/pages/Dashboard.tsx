import { useEffect, useState } from 'react'
import { authAPI, AdminUser } from '../api/client'

export function Dashboard() {
  const [user, setUser] = useState<AdminUser | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    authAPI
      .me()
      .then(setUser)
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <div>
        <h1>Dashboard</h1>
        <p aria-busy="true">Loading...</p>
      </div>
    )
  }

  return (
    <div>
      <h1>Dashboard</h1>
      
      {user && (
        <article>
          <hgroup>
            <h2>Welcome back!</h2>
            <p>You are logged in as <strong>{user.email || user.admin_id}</strong></p>
          </hgroup>
          
          <ul>
            <li><strong>Admin ID:</strong> {user.admin_id}</li>
            <li><strong>Auth Type:</strong> {user.auth_type}</li>
            <li><strong>Roles:</strong> {user.roles.join(', ')}</li>
          </ul>
        </article>
      )}

      <section>
        <h3>Quick Stats</h3>
        <p><em>TODO: Add dashboard stats (API key count, model count, billing summary, etc.)</em></p>
      </section>
    </div>
  )
}
