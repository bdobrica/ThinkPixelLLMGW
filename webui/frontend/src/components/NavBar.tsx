import { Link, useNavigate } from 'react-router-dom'
import { authAPI } from '../api/client'

export function NavBar() {
  const navigate = useNavigate()

  const handleLogout = async () => {
    try {
      await authAPI.logout()
      navigate('/login')
    } catch (err) {
      console.error('Logout failed:', err)
    }
  }

  return (
    <nav className="container">
      <ul>
        <li>
          <strong>LLM Gateway</strong>
        </li>
      </ul>
      <ul>
        <li>
          <Link to="/dashboard">Dashboard</Link>
        </li>
        <li>
          <Link to="/api-keys">API Keys</Link>
        </li>
        <li>
          <Link to="/models">Models</Link>
        </li>
        <li>
          <Link to="/billing">Billing</Link>
        </li>
        <li>
          <button onClick={handleLogout} className="outline">
            Logout
          </button>
        </li>
      </ul>
    </nav>
  )
}
