import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import { GetUnhealthyFeedsResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import FeedsCard from '@/components/monitoring/FeedsCard'

describe('FeedsCard', () => {
  it('shows a loading state without data', () => {
    render(<FeedsCard data={undefined} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows an all-healthy message when there are no unhealthy feeds', () => {
    const data = create(GetUnhealthyFeedsResponseSchema, { feeds: [] })
    render(<FeedsCard data={data} />)
    expect(screen.getByText('All feeds are healthy.')).toBeInTheDocument()
  })

  it('lists unhealthy feeds with their error and failure count', () => {
    const data = create(GetUnhealthyFeedsResponseSchema, {
      feeds: [
        {
          title: 'Broken Feed',
          url: 'https://example.com/feed.xml',
          lastError: 'timeout fetching feed',
          consecutiveFailures: 5
        }
      ]
    })

    render(<FeedsCard data={data} />)
    expect(screen.getByText('Broken Feed')).toBeInTheDocument()
    expect(screen.getByText('timeout fetching feed')).toBeInTheDocument()
    expect(screen.getByText('5 failure(s)')).toBeInTheDocument()
  })
})
