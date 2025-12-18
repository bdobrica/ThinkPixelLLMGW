import { useEffect, useState } from 'react'
import { adminAPI, ApiKey, Model, CreateApiKeyRequest, UpdateApiKeyRequest } from '../api/client'

export function ApiKeys() {
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [models, setModels] = useState<Model[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [page, setPage] = useState(1)
  const [totalCount, setTotalCount] = useState(0)
  
  // Dialog states
  const [showCreateDialog, setShowCreateDialog] = useState(false)
  const [showEditDialog, setShowEditDialog] = useState(false)
  const [showRevokeDialog, setShowRevokeDialog] = useState(false)
  const [selectedKey, setSelectedKey] = useState<ApiKey | null>(null)
  const [newKeyResponse, setNewKeyResponse] = useState<string>('')

  const loadApiKeys = () => {
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
  }

  const loadModels = () => {
    adminAPI
      .listModels(1, 100) // Load all models
      .then((response) => {
        setModels(response.items.filter(m => m.enabled))
      })
      .catch((err) => {
        console.error('Failed to load models:', err)
      })
  }

  useEffect(() => {
    loadApiKeys()
    loadModels()
  }, [page])

  const handleCreateKey = () => {
    setShowCreateDialog(true)
    setNewKeyResponse('')
  }

  const handleEditKey = (key: ApiKey) => {
    setSelectedKey(key)
    setShowEditDialog(true)
  }

  const handleRevokeKey = (key: ApiKey) => {
    setSelectedKey(key)
    setShowRevokeDialog(true)
  }

  const confirmRevoke = async () => {
    if (!selectedKey) return
    
    try {
      await adminAPI.revokeApiKey(selectedKey.id)
      setShowRevokeDialog(false)
      setSelectedKey(null)
      loadApiKeys()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to revoke API key')
    }
  }

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
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <hgroup style={{ margin: 0 }}>
          <h1>API Keys</h1>
          <p>Total: {totalCount}</p>
        </hgroup>
        <button onClick={handleCreateKey}>Create API Key</button>
      </div>

      {keys.length === 0 ? (
        <p><em>No API keys found</em></p>
      ) : (
        <figure>
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Models</th>
                <th>Enabled</th>
                <th>Rate Limit</th>
                <th>Budget</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {keys.map((key) => (
                <tr key={key.id}>
                  <td>{key.name}</td>
                  <td>
                    {key.allowed_models && key.allowed_models.length > 0 
                      ? key.allowed_models.join(', ')
                      : 'All'}
                  </td>
                  <td>{key.enabled ? '✓' : '✗'}</td>
                  <td>{key.rate_limit_per_minute}/min</td>
                  <td>
                    {key.monthly_budget_usd 
                      ? `$${key.monthly_budget_usd.toFixed(2)}` 
                      : 'Unlimited'}
                  </td>
                  <td>{new Date(key.created_at).toLocaleDateString()}</td>
                  <td>
                    <button 
                      onClick={() => handleEditKey(key)}
                      style={{ marginRight: '0.5rem' }}
                      className="secondary"
                    >
                      Edit
                    </button>
                    <button 
                      onClick={() => handleRevokeKey(key)}
                      className="contrast"
                    >
                      Revoke
                    </button>
                  </td>
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

      {/* Create API Key Dialog */}
      {showCreateDialog && (
        <CreateApiKeyDialog
          models={models}
          newKeyResponse={newKeyResponse}
          onClose={() => {
            setShowCreateDialog(false)
            setNewKeyResponse('')
          }}
          onCreate={async (request) => {
            try {
              const response = await adminAPI.createApiKey(request)
              setNewKeyResponse(response.key)
              loadApiKeys()
            } catch (err) {
              alert(err instanceof Error ? err.message : 'Failed to create API key')
            }
          }}
        />
      )}

      {/* Edit API Key Dialog */}
      {showEditDialog && selectedKey && (
        <EditApiKeyDialog
          apiKey={selectedKey}
          models={models}
          onClose={() => {
            setShowEditDialog(false)
            setSelectedKey(null)
          }}
          onUpdate={async (request) => {
            try {
              await adminAPI.updateApiKey(selectedKey.id, request)
              setShowEditDialog(false)
              setSelectedKey(null)
              loadApiKeys()
            } catch (err) {
              alert(err instanceof Error ? err.message : 'Failed to update API key')
            }
          }}
        />
      )}

      {/* Revoke API Key Dialog */}
      {showRevokeDialog && selectedKey && (
        <dialog open>
          <article>
            <header>
              <button 
                aria-label="Close" 
                className="close" 
                onClick={() => {
                  setShowRevokeDialog(false)
                  setSelectedKey(null)
                }}
              />
              <h3>Revoke API Key</h3>
            </header>
            <p>
              Are you sure you want to revoke the API key "<strong>{selectedKey.name}</strong>"?
              This action cannot be undone and will disable the key immediately.
            </p>
            <footer>
              <button 
                className="secondary" 
                onClick={() => {
                  setShowRevokeDialog(false)
                  setSelectedKey(null)
                }}
              >
                Cancel
              </button>
              <button onClick={confirmRevoke}>Revoke Key</button>
            </footer>
          </article>
        </dialog>
      )}
    </div>
  )
}

// Create API Key Dialog Component
interface CreateApiKeyDialogProps {
  models: Model[]
  newKeyResponse: string
  onClose: () => void
  onCreate: (request: CreateApiKeyRequest) => Promise<void>
}

function CreateApiKeyDialog({ models, newKeyResponse, onClose, onCreate }: CreateApiKeyDialogProps) {
  const [name, setName] = useState('')
  const [selectedModels, setSelectedModels] = useState<string[]>([])
  const [rateLimit, setRateLimit] = useState('60')
  const [monthlyBudget, setMonthlyBudget] = useState('')
  const [tags, setTags] = useState<Record<string, string>>({})
  const [newTagKey, setNewTagKey] = useState('')
  const [newTagValue, setNewTagValue] = useState('')

  const handleModelToggle = (modelName: string) => {
    setSelectedModels(prev => 
      prev.includes(modelName) 
        ? prev.filter(m => m !== modelName)
        : [...prev, modelName]
    )
  }

  const addTag = () => {
    if (newTagKey && newTagValue) {
      setTags(prev => ({ ...prev, [newTagKey]: newTagValue }))
      setNewTagKey('')
      setNewTagValue('')
    }
  }

  const removeTag = (key: string) => {
    setTags(prev => {
      const newTags = { ...prev }
      delete newTags[key]
      return newTags
    })
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    
    const request: CreateApiKeyRequest = {
      name,
      allowed_models: selectedModels.length > 0 ? selectedModels : undefined,
      rate_limit_per_minute: parseInt(rateLimit),
      monthly_budget_usd: monthlyBudget ? parseFloat(monthlyBudget) : undefined,
      tags: Object.keys(tags).length > 0 ? tags : undefined,
    }

    await onCreate(request)
  }

  if (newKeyResponse) {
    return (
      <dialog open>
        <article>
          <header>
            <button aria-label="Close" className="close" onClick={onClose} />
            <h3>API Key Created</h3>
          </header>
          <p>Your API key has been created successfully. Please copy it now - you won't be able to see it again!</p>
          <article style={{ backgroundColor: 'var(--pico-code-background-color)', padding: '1rem' }}>
            <code style={{ wordBreak: 'break-all' }}>{newKeyResponse}</code>
          </article>
          <footer>
            <button onClick={onClose}>Close</button>
          </footer>
        </article>
      </dialog>
    )
  }

  return (
    <dialog open>
      <article style={{ maxWidth: '600px' }}>
        <header>
          <button aria-label="Close" className="close" onClick={onClose} />
          <h3>Create API Key</h3>
        </header>
        <form onSubmit={handleSubmit}>
          <label>
            Name *
            <input 
              type="text" 
              value={name} 
              onChange={(e) => setName(e.target.value)}
              required 
              placeholder="My API Key"
            />
          </label>

          <label>
            Allowed Models (leave empty for all)
            <div style={{ maxHeight: '150px', overflowY: 'auto', border: '1px solid var(--pico-form-element-border-color)', padding: '0.5rem', borderRadius: '4px' }}>
              {models.map(model => (
                <label key={model.id} style={{ display: 'block', marginBottom: '0.25rem' }}>
                  <input
                    type="checkbox"
                    checked={selectedModels.includes(model.name)}
                    onChange={() => handleModelToggle(model.name)}
                  />
                  {model.name} ({model.provider_name})
                </label>
              ))}
            </div>
          </label>

          <label>
            Rate Limit (requests/minute) *
            <input 
              type="number" 
              value={rateLimit} 
              onChange={(e) => setRateLimit(e.target.value)}
              required 
              min="1"
            />
          </label>

          <label>
            Monthly Budget (USD, optional)
            <input 
              type="number" 
              value={monthlyBudget} 
              onChange={(e) => setMonthlyBudget(e.target.value)}
              step="0.01"
              min="0"
              placeholder="Leave empty for unlimited"
            />
          </label>

          <fieldset>
            <legend>Tags</legend>
            {Object.entries(tags).map(([key, value]) => (
              <div key={key} style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.5rem' }}>
                <input type="text" value={key} disabled style={{ flex: 1 }} />
                <input type="text" value={value} disabled style={{ flex: 1 }} />
                <button type="button" onClick={() => removeTag(key)} className="contrast">Remove</button>
              </div>
            ))}
            <div style={{ display: 'flex', gap: '0.5rem' }}>
              <input 
                type="text" 
                value={newTagKey} 
                onChange={(e) => setNewTagKey(e.target.value)}
                placeholder="Key"
                style={{ flex: 1 }}
              />
              <input 
                type="text" 
                value={newTagValue} 
                onChange={(e) => setNewTagValue(e.target.value)}
                placeholder="Value"
                style={{ flex: 1 }}
              />
              <button type="button" onClick={addTag} className="secondary">Add</button>
            </div>
          </fieldset>

          <footer>
            <button type="button" className="secondary" onClick={onClose}>Cancel</button>
            <button type="submit">Create</button>
          </footer>
        </form>
      </article>
    </dialog>
  )
}

// Edit API Key Dialog Component
interface EditApiKeyDialogProps {
  apiKey: ApiKey
  models: Model[]
  onClose: () => void
  onUpdate: (request: UpdateApiKeyRequest) => Promise<void>
}

function EditApiKeyDialog({ apiKey, models, onClose, onUpdate }: EditApiKeyDialogProps) {
  const [name, setName] = useState(apiKey.name)
  const [selectedModels, setSelectedModels] = useState<string[]>(apiKey.allowed_models || [])
  const [rateLimit, setRateLimit] = useState(apiKey.rate_limit_per_minute.toString())
  const [monthlyBudget, setMonthlyBudget] = useState(apiKey.monthly_budget_usd?.toString() || '')
  const [tags, setTags] = useState<Record<string, string>>(apiKey.tags || {})
  const [newTagKey, setNewTagKey] = useState('')
  const [newTagValue, setNewTagValue] = useState('')

  const handleModelToggle = (modelName: string) => {
    setSelectedModels(prev => 
      prev.includes(modelName) 
        ? prev.filter(m => m !== modelName)
        : [...prev, modelName]
    )
  }

  const addTag = () => {
    if (newTagKey && newTagValue) {
      setTags(prev => ({ ...prev, [newTagKey]: newTagValue }))
      setNewTagKey('')
      setNewTagValue('')
    }
  }

  const removeTag = (key: string) => {
    setTags(prev => {
      const newTags = { ...prev }
      delete newTags[key]
      return newTags
    })
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    
    const request: UpdateApiKeyRequest = {
      name,
      allowed_models: selectedModels,
      rate_limit_per_minute: parseInt(rateLimit),
      monthly_budget_usd: monthlyBudget ? parseFloat(monthlyBudget) : undefined,
      tags,
    }

    await onUpdate(request)
  }

  return (
    <dialog open>
      <article style={{ maxWidth: '600px' }}>
        <header>
          <button aria-label="Close" className="close" onClick={onClose} />
          <h3>Edit API Key</h3>
        </header>
        <form onSubmit={handleSubmit}>
          <label>
            Name *
            <input 
              type="text" 
              value={name} 
              onChange={(e) => setName(e.target.value)}
              required 
            />
          </label>

          <label>
            Allowed Models (leave empty for all)
            <div style={{ maxHeight: '150px', overflowY: 'auto', border: '1px solid var(--pico-form-element-border-color)', padding: '0.5rem', borderRadius: '4px' }}>
              {models.map(model => (
                <label key={model.id} style={{ display: 'block', marginBottom: '0.25rem' }}>
                  <input
                    type="checkbox"
                    checked={selectedModels.includes(model.name)}
                    onChange={() => handleModelToggle(model.name)}
                  />
                  {model.name} ({model.provider_name})
                </label>
              ))}
            </div>
          </label>

          <label>
            Rate Limit (requests/minute) *
            <input 
              type="number" 
              value={rateLimit} 
              onChange={(e) => setRateLimit(e.target.value)}
              required 
              min="1"
            />
          </label>

          <label>
            Monthly Budget (USD, optional)
            <input 
              type="number" 
              value={monthlyBudget} 
              onChange={(e) => setMonthlyBudget(e.target.value)}
              step="0.01"
              min="0"
              placeholder="Leave empty for unlimited"
            />
          </label>

          <fieldset>
            <legend>Tags</legend>
            {Object.entries(tags).map(([key, value]) => (
              <div key={key} style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.5rem' }}>
                <input type="text" value={key} disabled style={{ flex: 1 }} />
                <input type="text" value={value} disabled style={{ flex: 1 }} />
                <button type="button" onClick={() => removeTag(key)} className="contrast">Remove</button>
              </div>
            ))}
            <div style={{ display: 'flex', gap: '0.5rem' }}>
              <input 
                type="text" 
                value={newTagKey} 
                onChange={(e) => setNewTagKey(e.target.value)}
                placeholder="Key"
                style={{ flex: 1 }}
              />
              <input 
                type="text" 
                value={newTagValue} 
                onChange={(e) => setNewTagValue(e.target.value)}
                placeholder="Value"
                style={{ flex: 1 }}
              />
              <button type="button" onClick={addTag} className="secondary">Add</button>
            </div>
          </fieldset>

          <footer>
            <button type="button" className="secondary" onClick={onClose}>Cancel</button>
            <button type="submit">Update</button>
          </footer>
        </form>
      </article>
    </dialog>
  )
}
