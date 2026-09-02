import React from 'react'
import { render, screen } from '@testing-library/react'

const ROUTES = [
  'games',
  'books',
  'feeds',
  'recipes/list',
  'mealplans',
  'shoppinglist',
  'contacts',
  'family',
  'user-management',
  'monitoring'
]

describe.each(ROUTES)('app/%s/loading.tsx', (route) => {
  it('renders the shared PageLoading fallback', async () => {
    const { default: Loading } = await import(`@/app/${route}/loading`)
    render(<Loading />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })
})
