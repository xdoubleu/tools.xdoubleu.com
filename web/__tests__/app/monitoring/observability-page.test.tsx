import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/monitoring/ObservabilityClient', () => () => (
  <div data-testid="observability-client" />
))

import MonitoringObservabilityPage from '@/app/monitoring/observability/page'

describe('MonitoringObservabilityPage', () => {
  it('renders the observability client with no server-side data prefetch', () => {
    // Issue #1714: this page no longer awaits GetAutomatedActions during
    // SSR (see the comment in app/monitoring/observability/page.tsx), so
    // rendering it needs no fetchOrNull/SWRFallback mocking at all —
    // ObservabilityClient's own SWR hook (covered by
    // ObservabilityClient.test.tsx) owns the client-side fetch and loading
    // state.
    render(<MonitoringObservabilityPage />)
    expect(screen.getByTestId('observability-client')).toBeInTheDocument()
  })
})
