import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import { GetUsageStatsResponseSchema } from '@/lib/gen/admin/v1/admin_pb'
import UsageCard from '@/components/admin/observability/UsageCard'

jest.mock('recharts', () => {
  const Original = jest.requireActual('recharts')
  return {
    ...Original,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div style={{ width: 400, height: 300 }}>{children}</div>
    )
  }
})

describe('UsageCard', () => {
  it('renders the endpoint drill-down table', () => {
    const data = create(GetUsageStatsResponseSchema, {
      entries: [
        { day: '2026-01-01', app: 'books', endpoint: 'LibraryService/List', count: 12n },
        { day: '2026-01-01', app: 'games', endpoint: 'GamesService/List', count: 4n }
      ]
    })

    render(<UsageCard data={data} />)
    expect(screen.getByText('LibraryService/List')).toBeInTheDocument()
    expect(screen.getByText('GamesService/List')).toBeInTheDocument()
  })

  it('shows an empty state without data', () => {
    render(<UsageCard data={undefined} />)
    expect(screen.getByText('No usage recorded yet.')).toBeInTheDocument()
  })
})
