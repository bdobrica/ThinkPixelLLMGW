/**
 * Simple API client for calling the BFF.
 * All requests go through the BFF, which handles auth cookies.
 */

const API_BASE = '' // Same origin, Vite proxies /auth and /admin to BFF

export interface LoginCredentials {
  email: string
  password: string
}

export interface ApiKey {
  id: string
  name: string
  allowed_models: string[]
  rate_limit_per_minute: number
  monthly_budget_usd?: number
  enabled: boolean
  expires_at?: string
  created_at: string
  updated_at: string
}

export interface ApiKeysResponse {
  items: ApiKey[]
  total_count: number
  page: number
  page_size: number
}

export interface AdminUser {
  admin_id: string
  email?: string
  roles: string[]
  auth_type: string
}

/**
 * Generic fetch wrapper that throws on non-ok responses.
 */
async function fetchJSON<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    ...options,
    credentials: 'include', // Important: send cookies
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  })

  if (!response.ok) {
    const error = await response.json().catch(() => ({ detail: 'Request failed' }))
    throw new Error(error.detail || `HTTP ${response.status}`)
  }

  return response.json()
}

/**
 * Auth API
 */
export const authAPI = {
  async login(credentials: LoginCredentials): Promise<{ success: boolean }> {
    return fetchJSON(`${API_BASE}/auth/login`, {
      method: 'POST',
      body: JSON.stringify(credentials),
    })
  },

  async logout(): Promise<{ success: boolean }> {
    return fetchJSON(`${API_BASE}/auth/logout`, {
      method: 'POST',
    })
  },

  async me(): Promise<AdminUser> {
    return fetchJSON(`${API_BASE}/auth/me`)
  },
}

/**
 * Admin API
 */
export const adminAPI = {
  async listApiKeys(page = 1, pageSize = 20): Promise<ApiKeysResponse> {
    return fetchJSON(`${API_BASE}/admin/api-keys?page=${page}&page_size=${pageSize}`)
  },

  async listModels(page = 1, pageSize = 20): Promise<any> {
    return fetchJSON(`${API_BASE}/admin/models?page=${page}&page_size=${pageSize}`)
  },

  async getBilling(): Promise<any> {
    return fetchJSON(`${API_BASE}/admin/billing`)
  },
}
