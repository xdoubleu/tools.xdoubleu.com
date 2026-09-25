import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/monitoring/ObservabilityClient', () => () => (
  <div data-testid="observability-client" />
))

import MonitoringObservabilityPage from '@/app/monitoring/observability/page'

describe('MonitoringObservabilityPage', () => {
  it('renders the observability client with no server-side data prefetch', () => {
    // No SSR prefetch, so no fetchOrNull/SWRFallback mocking is needed.
    render(<MonitoringObservabilityPage />)
    expect(screen.getByTestId('observability-client')).toBeInTheDocument()
  })
})
