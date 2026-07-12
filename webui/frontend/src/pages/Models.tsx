import { FormEvent, useEffect, useState } from 'react'
import { adminAPI, Model, PricingComponent } from '../api/client'

const PAGE_SIZE = 12

function formatLimit(value: number) {
  return value > 0 ? value.toLocaleString() : 'Not specified'
}

function formatPrice(component: PricingComponent, currency: string) {
  return `${component.price.toLocaleString(undefined, { maximumFractionDigits: 12 })} ${currency} / ${component.unit.replace(/_/g, ' ')}`
}

export function Models() {
  const [models, setModels] = useState<Model[]>([])
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [query, setQuery] = useState('')
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    setLoading(true)
    setError('')
    adminAPI.listModels(page, PAGE_SIZE, search)
      .then((response) => {
        if (!active) return
        setModels(response.items)
        setTotal(response.total_count)
      })
      .catch((reason: unknown) => {
        if (active) setError(reason instanceof Error ? reason.message : 'Failed to load models')
      })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [page, search])

  const submitSearch = (event: FormEvent) => {
    event.preventDefault()
    setPage(1)
    setSearch(query.trim())
  }

  const lastPage = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <div>
      <hgroup>
        <h1>Models</h1>
        <p>Read-only model catalog. Model changes remain available through the admin API to administrators.</p>
      </hgroup>

      <form role="search" onSubmit={submitSearch}>
        <input aria-label="Search models" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search by model name" />
        <button type="submit">Search</button>
      </form>

      {error && <article role="alert"><strong>Models could not be loaded.</strong> {error} <button className="outline" onClick={() => setSearch((value) => value + ' ')}>Retry</button></article>}
      {loading && <p aria-busy="true">Loading models…</p>}
      {!loading && !error && models.length === 0 && <article>No models match this view.</article>}

      {!loading && !error && models.map((model) => (
        <article key={model.id}>
          <header style={{ display: 'flex', justifyContent: 'space-between', gap: '1rem' }}>
            <div><strong>{model.model_name}</strong><br /><small>{model.provider_name} · {model.source}{model.version ? ` · ${model.version}` : ''}</small></div>
            <span>{model.is_deprecated ? 'Deprecated' : 'Available'}</span>
          </header>
          <div className="grid">
            <section><strong>Capabilities</strong><p>{model.features.length ? model.features.join(', ') : 'None declared'}</p></section>
            <section><strong>Token limits</strong><p>Input: {formatLimit(model.max_input_tokens)}<br />Output: {formatLimit(model.max_output_tokens)}</p></section>
            <section><strong>Pricing</strong>{model.pricing_components.length ? <ul>{model.pricing_components.map((price) => <li key={price.id}>{price.direction} {price.modality}: {formatPrice(price, model.currency)}</li>)}</ul> : <p>Not configured</p>}</section>
          </div>
        </article>
      ))}

      {!loading && !error && total > 0 && (
        <nav aria-label="Model pages">
          <ul><li><button className="outline" disabled={page <= 1} onClick={() => setPage((value) => value - 1)}>Previous</button></li></ul>
          <ul><li>Page {page} of {lastPage} · {total} models</li><li><button className="outline" disabled={page >= lastPage} onClick={() => setPage((value) => value + 1)}>Next</button></li></ul>
        </nav>
      )}
    </div>
  )
}
