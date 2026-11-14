import React from 'react'
import { render, screen } from '@testing-library/react'

vi.mock('@/components/factory/RoutinesClient', () => ({
  __esModule: true,
  default: () => <div data-testid="routines" />
}))

import FactoryRoutinesPage from '@/app/factory/routines/page'

describe('FactoryRoutinesPage', () => {
  it('renders the routines', () => {
    render(<FactoryRoutinesPage />)
    expect(screen.getByTestId('routines')).toBeInTheDocument()
  })
})
