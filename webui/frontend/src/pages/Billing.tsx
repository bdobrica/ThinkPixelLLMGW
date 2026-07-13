import { useEffect, useState } from 'react'
import { adminAPI, BillingResponse, UsageResponse } from '../api/client'

const PAGE_SIZE = 20
const formatNumber = (value: number) => value.toLocaleString()
const formatCost = (value: number) => `${value.toFixed(6)} USD`

export function Billing() {
  const now = new Date()
  const [year, setYear] = useState(now.getUTCFullYear())
  const [month, setMonth] = useState(now.getUTCMonth() + 1)
  const [page, setPage] = useState(1)
  const [billing, setBilling] = useState<BillingResponse | null>(null)
  const [usage, setUsage] = useState<UsageResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true; setLoading(true); setError('')
    const start = new Date(Date.UTC(year, month - 1, 1)).toISOString()
    const end = new Date(Date.UTC(year, month, 1)).toISOString()
    Promise.all([adminAPI.listMonthlyBilling(page, PAGE_SIZE, year, month), adminAPI.listUsage(1, 10, start, end)])
      .then(([billingData, usageData]) => { if (active) { setBilling(billingData); setUsage(usageData) } })
      .catch((reason: unknown) => { if (active) setError(reason instanceof Error ? reason.message : 'Failed to load billing') })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [year, month, page])

  const pages = Math.max(1, Math.ceil((billing?.total_count ?? 0) / PAGE_SIZE))
  return <div>
    <hgroup><h1>Billing and usage</h1><p>Persisted monthly USD costs and request usage. Times are UTC; totals reflect acknowledged queue records.</p></hgroup>
    <div className="grid"><label>Year<input type="number" min="2000" max="9999" value={year} onChange={(event) => { setYear(Number(event.target.value)); setPage(1) }} /></label><label>Month<select value={month} onChange={(event) => { setMonth(Number(event.target.value)); setPage(1) }}>{Array.from({length:12},(_,index)=><option key={index+1} value={index+1}>{new Date(Date.UTC(2020,index)).toLocaleString(undefined,{month:'long',timeZone:'UTC'})}</option>)}</select></label></div>
    {error && <article role="alert"><strong>Billing could not be loaded.</strong> {error}</article>}
    {loading && <p aria-busy="true">Loading billing and usage…</p>}
    {!loading && !error && billing && <>
      <section className="grid"><article><small>Page cost</small><h2>{formatCost(billing.page_totals.cost_usd)}</h2></article><article><small>Page requests</small><h2>{formatNumber(billing.page_totals.requests)}</h2></article><article><small>Page tokens</small><h2>{formatNumber(billing.page_totals.tokens)}</h2></article></section>
      <h2>Monthly totals by API key</h2>
      {billing.items.length === 0 ? <article>No persisted billing summaries for this month.</article> : <div style={{overflowX:'auto'}}><table><thead><tr><th>API key</th><th>Requests</th><th>Tokens</th><th>Cost</th></tr></thead><tbody>{billing.items.map(item=><tr key={item.api_key_id}><td>{item.api_key_name}</td><td>{formatNumber(item.total_requests)}</td><td>{formatNumber(item.total_tokens)}</td><td>{formatCost(item.total_cost_usd)}</td></tr>)}</tbody></table></div>}
      {billing.total_count>0 && <nav aria-label="Billing pages"><ul><li><button className="outline" disabled={page<=1} onClick={()=>setPage(value=>value-1)}>Previous</button></li></ul><ul><li>Page {page} of {pages}</li><li><button className="outline" disabled={page>=pages} onClick={()=>setPage(value=>value+1)}>Next</button></li></ul></nav>}
      <h2>Recent requests in selected month</h2>
      {!usage?.items.length ? <article>No usage records for this month.</article> : <div style={{overflowX:'auto'}}><table><thead><tr><th>Time (UTC)</th><th>API key</th><th>Model</th><th>Status</th><th>Tokens</th><th>Latency</th></tr></thead><tbody>{usage.items.map(item=><tr key={item.id}><td>{new Date(item.created_at).toISOString()}</td><td>{item.api_key_name}</td><td>{item.model_name}</td><td>{item.status_code}</td><td>{formatNumber(item.input_tokens+item.output_tokens+item.cached_tokens+item.reasoning_tokens)}</td><td>{formatNumber(item.response_time_ms)} ms</td></tr>)}</tbody></table></div>}
    </>}
  </div>
}
