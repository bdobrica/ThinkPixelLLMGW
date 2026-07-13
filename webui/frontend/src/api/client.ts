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
  tags?: Record<string, string>
  created_at: string
  updated_at: string
}

export interface CreateApiKeyRequest {
  name: string
  allowed_models?: string[]
  rate_limit_per_minute: number
  monthly_budget_usd?: number
  enabled?: boolean
  expires_at?: string
  tags?: Record<string, string>
}

export interface UpdateApiKeyRequest {
  name?: string
  allowed_models?: string[]
  rate_limit_per_minute?: number
  monthly_budget_usd?: number
  enabled?: boolean
  expires_at?: string
  tags?: Record<string, string>
}

export interface ApiKeyCreatedResponse extends ApiKey {
  key: string // Plaintext key - only returned once
}

export interface Model {
  id: string
  model_name: string
  provider_id: string
  provider_name: string
  source: string
  version?: string
  is_deprecated: boolean
  currency: string
  features: string[]
  max_input_tokens: number
  max_output_tokens: number
  pricing_components: PricingComponent[]
  created_at: string
  updated_at: string
}

export interface PricingComponent {
  id: string
  code: string
  direction: string
  modality: string
  unit: string
  tier?: string
  scope?: string
  price: number
}

export interface ApiKeysResponse {
  items: ApiKey[]
  total_count: number
  page: number
  page_size: number
}

export interface ModelsResponse {
  items: Model[]
  total_count: number
  page: number
  page_size: number
}

export interface AdminUser {
  admin_id: string
  email?: string
  roles: string[]
  auth_type: string
  service_name?: string
}

export interface UsageRecord {
  id: string; api_key_id: string; api_key_name: string; request_id: string; model_name: string; endpoint: string
  input_tokens: number; output_tokens: number; cached_tokens: number; reasoning_tokens: number
  response_time_ms: number; status_code: number; created_at: string
}
export interface UsageResponse { items: UsageRecord[]; total_count: number; page: number; page_size: number; start: string; end: string }
export interface MonthlyBilling {
  api_key_id: string; api_key_name: string; year: number; month: number; total_requests: number
  total_input_tokens: number; total_output_tokens: number; total_cached_tokens: number; total_reasoning_tokens: number
  total_tokens: number; total_cost_usd: number; currency: string
}
export interface BillingResponse {
  items: MonthlyBilling[]; total_count: number; page: number; page_size: number; year: number; month: number
  page_totals: { requests: number; tokens: number; cost_usd: number; currency: string }
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

  async createApiKey(request: CreateApiKeyRequest): Promise<ApiKeyCreatedResponse> {
    return fetchJSON(`${API_BASE}/admin/api-keys`, {
      method: 'POST',
      body: JSON.stringify(request),
    })
  },

  async updateApiKey(keyId: string, request: UpdateApiKeyRequest): Promise<ApiKey> {
    return fetchJSON(`${API_BASE}/admin/api-keys/${keyId}`, {
      method: 'PUT',
      body: JSON.stringify(request),
    })
  },

  async revokeApiKey(keyId: string): Promise<{ success: boolean }> {
    return fetchJSON(`${API_BASE}/admin/api-keys/${keyId}`, {
      method: 'DELETE',
    })
  },

  async listModels(page = 1, pageSize = 20, search = ''): Promise<ModelsResponse> {
    const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
    if (search.trim()) params.set('search', search.trim())
    return fetchJSON(`${API_BASE}/admin/models?${params}`)
  },

  async listUsage(page = 1, pageSize = 20, start?: string, end?: string): Promise<UsageResponse> {
    const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
    if (start) params.set('start', start)
    if (end) params.set('end', end)
    return fetchJSON(`${API_BASE}/admin/usage?${params}`)
  },

  async listMonthlyBilling(page = 1, pageSize = 20, year?: number, month?: number): Promise<BillingResponse> {
    const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
    if (year) params.set('year', String(year))
    if (month) params.set('month', String(month))
    return fetchJSON(`${API_BASE}/admin/billing/monthly?${params}`)
  },
}
