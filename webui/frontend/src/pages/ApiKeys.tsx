import { useEffect, useState } from 'react'
import { adminAPI, ApiKey } from '../api/client'

export function ApiKeys() {
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [page, setPage] = useState(1)
  const [totalCount, setTotalCount] = useState(0)

  useEffect(() => {
    setLoading(true)
    setError('')
    
    adminAPI
      .listApiKeys(page, 20)
      .then((response) => {
        setKeys(response.items)
        setTotalCount(response.total_count)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'Failed to load API keys')
      })
      .finally(() => setLoading(false))
  }, [page])

  if (loading) {
    return (
      <div>
        <h1>API Keys</h1>
        <p aria-busy="true">Loading...</p>
      </div>
    )
  }

  if (error) {
    return (
      <div>
        <h1>API Keys</h1>
        <article style={{ backgroundColor: 'var(--pico-del-background)' }}>
          <p>{error}</p>
        </article>
      </div>
    )
  }

  return (
    <div>
      <hgroup>
        <h1>API Keys</h1>
        <p>Total: {totalCount}</p>
      </hgroup>

      {keys.length === 0 ? (
        <p><em>No API keys found</em></p>
      ) : (
        <figure>
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Enabled</th>
                <th>Rate Limit</th>
                <th>Budget</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {keys.map((key) => (
                <tr key={key.id}>
                  <td>{key.name}</td>
                  <td>{key.enabled ? '✓' : '✗'}</td>
                  <td>{key.rate_limit_per_minute}/min</td>
                  <td>
                    {key.monthly_budget_usd 
                      ? `$${key.monthly_budget_usd.toFixed(2)}` 
                      : 'Unlimited'}
                  </td>
                  <td>{new Date(key.created_at).toLocaleDateString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </figure>
      )}

      {totalCount > 20 && (
        <nav>
          <ul>
            <li>
              <button 
                disabled={page === 1} 
                onClick={() => setPage(p => p - 1)}
              >
                Previous
              </button>
            </li>
            <li>Page {page}</li>
            <li>
              <button 
                disabled={page * 20 >= totalCount} 
                onClick={() => setPage(p => p + 1)}
              >
                Next
              </button>
            </li>
          </ul>
        </nav>
      )}

      <section style={{ marginTop: '2rem' }}>
        <p><em>TODO: Add "Create API Key" button and form</em></p>
      </section>
    </div>
  )
}
