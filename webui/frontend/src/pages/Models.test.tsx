import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { adminAPI, Model } from '../api/client'
import { Models } from './Models'

const model: Model = {
  id: 'model-1', model_name: 'gpt-test', provider_id: 'provider-1', provider_name: 'openai',
  source: 'openai', is_deprecated: false, currency: 'USD', features: ['streaming output'],
  max_input_tokens: 128000, max_output_tokens: 4096, created_at: '2026-07-12T00:00:00Z',
  updated_at: '2026-07-12T00:00:00Z', pricing_components: [{
    id: 'price-1', code: 'input', direction: 'input', modality: 'text', unit: '1k_tokens', price: 0.005,
  }],
}

afterEach(() => vi.restoreAllMocks())

describe('Models', () => {
  it('renders the model hierarchy and pagination', async () => {
    vi.spyOn(adminAPI, 'listModels').mockResolvedValue({ items: [model], total_count: 13, page: 1, page_size: 12 })
    render(<Models />)

    expect(await screen.findByText('gpt-test')).toBeInTheDocument()
    expect(screen.getByText(/openai · openai/)).toBeInTheDocument()
    expect(screen.getByText(/128,000/)).toBeInTheDocument()
    expect(screen.getByText(/0.005 USD/)).toBeInTheDocument()
    expect(screen.getByText(/Page 1 of 2/)).toBeInTheDocument()
  })

  it('submits a bounded search and reports an empty result', async () => {
    const list = vi.spyOn(adminAPI, 'listModels')
      .mockResolvedValueOnce({ items: [model], total_count: 1, page: 1, page_size: 12 })
      .mockResolvedValueOnce({ items: [], total_count: 0, page: 1, page_size: 12 })
    render(<Models />)
    await screen.findByText('gpt-test')

    await userEvent.type(screen.getByLabelText('Search models'), 'missing')
    await userEvent.click(screen.getByRole('button', { name: 'Search' }))

    await screen.findByText('No models match this view.')
    await waitFor(() => expect(list).toHaveBeenLastCalledWith(1, 12, 'missing'))
  })
})
