import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/feeds/FeedsNotificationSettingsCard', () => () => (
  <div data-testid="feeds-notification-settings-card" />
))

jest.mock('@/components/feeds/FeedFilterRulesCard', () => () => (
  <div data-testid="feed-filter-rules-card" />
))

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({
    getNotificationSettings: jest.fn(async () => ({ settings: [] })),
    listFeeds: jest.fn(async () => ({ feeds: [] })),
    listFilterRules: jest.fn(async () => ({ rules: [] }))
  }))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

const mockFallback = jest.fn()
jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({
    fallback,
    children
  }: {
    fallback: Record<string, unknown>
    children: React.ReactNode
  }) => {
    mockFallback(fallback)
    return <>{children}</>
  }
}))

import FeedsSettingsPage from '@/app/feeds/settings/page'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'

describe('FeedsSettingsPage', () => {
  it('renders the feeds notification settings card', async () => {
    render(await FeedsSettingsPage())
    expect(screen.getByTestId('feeds-notification-settings-card')).toBeInTheDocument()
  })

  it('prefetches notification settings, feeds and filter rules into the SWR fallback', async () => {
    jest.mocked(fetchOrNull).mockImplementation(async (fn) => fn())
    render(await FeedsSettingsPage())
    expect(mockFallback).toHaveBeenCalledWith({
      [swrKeys.monitoringNotificationSettings]: { settings: [] },
      [swrKeys.feeds]: { feeds: [] },
      [swrKeys.feedFilterRules]: { rules: [] }
    })
  })

  it('leaves the fallback empty when prefetches fail', async () => {
    jest.mocked(fetchOrNull).mockResolvedValue(null)
    render(await FeedsSettingsPage())
    expect(mockFallback).toHaveBeenCalledWith({})
  })

  it('renders the filter rules card', async () => {
    render(await FeedsSettingsPage())
    expect(screen.getByTestId('feed-filter-rules-card')).toBeInTheDocument()
  })

  it('shows a breadcrumb back to /feeds', async () => {
    render(await FeedsSettingsPage())
    expect(screen.getByRole('link', { name: 'Feeds' })).toHaveAttribute('href', '/feeds')
    expect(screen.getByText('Settings')).toHaveAttribute('aria-current', 'page')
  })
})
