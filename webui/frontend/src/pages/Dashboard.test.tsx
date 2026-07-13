import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { adminAPI, authAPI } from '../api/client'
import { Dashboard } from './Dashboard'

afterEach(()=>vi.restoreAllMocks())

describe('Dashboard',()=>{
  it('renders bounded gateway summary semantics',async()=>{
    vi.spyOn(authAPI,'me').mockResolvedValue({admin_id:'admin-1',email:'viewer@example.test',roles:['viewer'],auth_type:'user'})
    vi.spyOn(adminAPI,'dashboard').mockResolvedValue({range:{start:'2026-07-11T00:00:00Z',end:'2026-07-12T00:00:00Z',hours:24},counts:{api_keys:3,models:4,providers:1},usage:{requests:10,errors:1,error_rate:.1,tokens:500,average_latency_ms:125},current_month:{cost_usd:.25,currency:'USD'},top_models:[{name:'gpt-test',requests:10,errors:1,tokens:500}],top_api_keys:[]})
    render(<Dashboard/>)
    expect(await screen.findByText(/viewer@example.test/)).toBeInTheDocument()
    expect(screen.getByText('10.00%')).toBeInTheDocument()
    expect(screen.getByText('0.250000 USD')).toBeInTheDocument()
    expect(screen.getByText('gpt-test')).toBeInTheDocument()
  })
})
