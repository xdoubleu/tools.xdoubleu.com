import React from 'react'
import { render, screen } from '@testing-library/react'

const fetchOrNull = jest.fn()
const fallbackProps = jest.fn()
const mockGetSeries = jest.fn()

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({ getSeries: mockGetSeries }))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: (fn: () => Promise<unknown>) => fetchOrNull(fn)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children, keyed }: { children: React.ReactNode; keyed: unknown }) => {
    fallbackProps(keyed)
    return <>{children}</>
  }
}))

jest.mock('@/components/books/SeriesBooksClient', () => ({
  __esModule: true,
  default: ({ name }: { name: string }) => <div data-testid="client">{name}</div>
}))

import Page from '@/app/books/series/[name]/page'

describe('SeriesPage', () => {
  beforeEach(() => fallbackProps.mockReset())

  it('decodes the name and seeds the series key', async () => {
    const data = { name: 'Discworld' }
    mockGetSeries.mockResolvedValue(data)
    fetchOrNull.mockImplementation((fn: () => Promise<unknown>) => fn())
    render(await Page({ params: Promise.resolve({ name: 'Disc%20world' }) }))
    expect(mockGetSeries).toHaveBeenCalledWith({ name: 'Disc world' })
    expect(screen.getByTestId('client')).toHaveTextContent('Disc world')
    expect(fallbackProps).toHaveBeenCalledWith([[['/books/series', 'Disc world'], data]])
  })

  it('renders without a fallback when the fetch fails', async () => {
    fetchOrNull.mockResolvedValue(null)
    render(await Page({ params: Promise.resolve({ name: 'Discworld' }) }))
    expect(screen.getByTestId('client')).toBeInTheDocument()
    expect(fallbackProps).toHaveBeenCalledWith([])
  })
})
