import React from 'react'
import { render, screen } from '@testing-library/react'

vi.mock('@/components/factory/ReposClient', () => ({
  __esModule: true,
  default: () => <div data-testid="repos" />
}))

import FactoryReposPage from '@/app/factory/repos/page'

describe('FactoryReposPage', () => {
  it('renders the repo settings', () => {
    render(<FactoryReposPage />)
    expect(screen.getByTestId('repos')).toBeInTheDocument()
  })
})
