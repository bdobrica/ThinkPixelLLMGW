import { useEffect, useState } from 'react'
import { adminAPI, authAPI, AdminUser, DashboardResponse } from '../api/client'

const number = (value: number) => value.toLocaleString()

export function Dashboard() {
  const [user, setUser] = useState<AdminUser | null>(null)
  const [data, setData] = useState<DashboardResponse | null>(null)
  const [hours, setHours] = useState(24)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let active=true; setLoading(true); setError('')
    Promise.all([authAPI.me(),adminAPI.dashboard(hours)]).then(([identity,summary])=>{if(active){setUser(identity);setData(summary)}}).catch((reason:unknown)=>{if(active)setError(reason instanceof Error?reason.message:'Failed to load dashboard')}).finally(()=>{if(active)setLoading(false)})
    return()=>{active=false}
  },[hours])

  return <div>
    <hgroup><h1>Dashboard</h1><p>{user ? `Signed in as ${user.email || user.admin_id} (${user.roles.join(', ')})` : 'Gateway administration overview'}</p></hgroup>
    <label>Usage window<select value={hours} onChange={event=>setHours(Number(event.target.value))}><option value={1}>Last hour</option><option value={24}>Last 24 hours</option><option value={168}>Last 7 days</option></select></label>
    {error&&<article role="alert"><strong>Dashboard could not be loaded.</strong> {error}</article>}
    {loading&&<p aria-busy="true">Loading gateway statistics…</p>}
    {!loading&&!error&&data&&<>
      <p><small>Usage range (UTC): {new Date(data.range.start).toISOString()} – {new Date(data.range.end).toISOString()}</small></p>
      <section className="grid"><article><small>Enabled API keys</small><h2>{number(data.counts.api_keys)}</h2></article><article><small>Available models</small><h2>{number(data.counts.models)}</h2></article><article><small>Enabled providers</small><h2>{number(data.counts.providers)}</h2></article></section>
      <section className="grid"><article><small>Requests</small><h2>{number(data.usage.requests)}</h2></article><article><small>Error rate</small><h2>{(data.usage.error_rate*100).toFixed(2)}%</h2><small>{number(data.usage.errors)} errors</small></article><article><small>Tokens</small><h2>{number(data.usage.tokens)}</h2></article><article><small>Average latency</small><h2>{number(Math.round(data.usage.average_latency_ms))} ms</h2></article><article><small>Current month cost</small><h2>{data.current_month.cost_usd.toFixed(6)} {data.current_month.currency}</h2></article></section>
      <div className="grid"><Ranking title="Top models" items={data.top_models}/><Ranking title="Top API keys" items={data.top_api_keys}/></div>
    </>}
  </div>
}

function Ranking({title,items}:{title:string;items:DashboardResponse['top_models']}) {
  return <article><h2>{title}</h2>{items.length===0?<p>No requests in this range.</p>:<table><thead><tr><th>Name</th><th>Requests</th><th>Errors</th></tr></thead><tbody>{items.map(item=><tr key={item.name}><td>{item.name}</td><td>{number(item.requests)}</td><td>{number(item.errors)}</td></tr>)}</tbody></table>}</article>
}
