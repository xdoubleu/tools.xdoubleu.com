import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/hooks/useICSProxy', () => ({
  useICSFeeds: jest.fn(),
  useDeleteConfig: jest.fn(() => jest.fn())
}))

jest.mock('@/lib/env', () => ({ getApiUrl: () => 'http://localhost' }))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

import FeedsListClient from '@/components/icsproxy/FeedsListClient'
import { useICSFeeds } from '@/hooks/useICSProxy'
import { create } from '@bufbuild/protobuf'
import {
  FilterConfigSchema,
  ListConfigsResponseSchema
} from '@/lib/gen/icsproxy/v1/proxy_pb'
import type { ListConfigsResponse } from '@/lib/gen/icsproxy/v1/proxy_pb'

function mockFeeds(value: { data?: ListConfigsResponse; error?: Error; isLoading: boolean }) {
  jest.mocked(useICSFeeds).mockReturnValue({
    data: value.data,
    error: value.error,
    isLoading: value.isLoading,
    isValidating: false,
    mutate: jest.fn(async () => undefined)
  })
}

beforeEach(() => jest.clearAllMocks())

describe('FeedsListClient', () => {
  it('shows a loading state', () => {
    mockFeeds({ isLoading: true })
    render(<FeedsListClient />)
    expect(screen.getByText('Loading feeds…')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    mockFeeds({ error: new Error('boom'), isLoading: false })
    render(<FeedsListClient />)
    expect(screen.getByText('Failed to load feeds.')).toBeInTheDocument()
  })

  it('shows an empty state when there are no configs', () => {
    mockFeeds({ data: create(ListConfigsResponseSchema, { configs: [] }), isLoading: false })
    render(<FeedsListClient />)
    expect(screen.getByText('No filter configs yet.')).toBeInTheDocument()
  })

  it('renders a card per config', () => {
    mockFeeds({
      data: create(ListConfigsResponseSchema, {
        configs: [create(FilterConfigSchema, { token: 't1', sourceUrl: 'https://cal.example/a.ics' })]
      }),
      isLoading: false
    })
    render(<FeedsListClient />)
    expect(screen.getByText('https://cal.example/a.ics')).toBeInTheDocument()
  })
})
