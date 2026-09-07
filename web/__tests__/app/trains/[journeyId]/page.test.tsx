import React from 'react'
import { render, screen } from '@testing-library/react'

const fetchOrNull = jest.fn()
const getJourneyDetail = jest.fn()

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({ getJourneyDetail }))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: (fn: () => Promise<unknown>) => fetchOrNull(fn)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

jest.mock('@/components/trains/JourneyDetailClient', () => ({
  __esModule: true,
  default: ({ journeyId }: { journeyId: string }) => <div data-testid="client">{journeyId}</div>
}))

import Page from '@/app/trains/[journeyId]/page'

describe('JourneyDetailPage', () => {
  it('renders with server-prefetched journey detail', async () => {
    fetchOrNull.mockImplementation(async (fn: () => Promise<unknown>) => fn())
    getJourneyDetail.mockResolvedValue({ journey: { legs: [] } })

    render(await Page({ params: Promise.resolve({ journeyId: 'journey-1' }) }))

    expect(screen.getByTestId('client')).toHaveTextContent('journey-1')
    expect(getJourneyDetail).toHaveBeenCalledWith({ journeyId: 'journey-1' })
  })

  it('still renders the client component when the server fetch fails', async () => {
    fetchOrNull.mockResolvedValue(null)

    render(await Page({ params: Promise.resolve({ journeyId: 'journey-2' }) }))

    expect(screen.getByTestId('client')).toHaveTextContent('journey-2')
  })
})
