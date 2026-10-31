import React from 'react'
import { render, screen } from '@testing-library/react'

vi.mock('@/components/factory/FactoryClient', () => ({
  __esModule: true,
  default: () => <div data-testid="board" />
}))

import FactoryPage from '@/app/factory/page'

describe('FactoryPage', () => {
  it('renders the board', () => {
    render(<FactoryPage />)
    expect(screen.getByTestId('board')).toBeInTheDocument()
  })
})
