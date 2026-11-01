import React from 'react'
import { render, screen } from '@testing-library/react'

vi.mock('@/components/factory/RunnersClient', () => ({
  __esModule: true,
  default: () => <div data-testid="runners" />
}))

import FactoryRunnersPage from '@/app/factory/runners/page'

describe('FactoryRunnersPage', () => {
  it('renders the runners', () => {
    render(<FactoryRunnersPage />)
    expect(screen.getByTestId('runners')).toBeInTheDocument()
  })
})
