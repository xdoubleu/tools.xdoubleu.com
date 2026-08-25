import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/feeds/FeedsNotificationSettingsCard', () => () => (
  <div data-testid="feeds-notification-settings-card" />
))

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({
    getNotificationSettings: jest.fn(async () => ({}))
  }))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

import FeedsSettingsPage from '@/app/feeds/settings/page'

describe('FeedsSettingsPage', () => {
  it('renders the feeds notification settings card', async () => {
    render(await FeedsSettingsPage())
    expect(screen.getByTestId('feeds-notification-settings-card')).toBeInTheDocument()
  })

  it('links back to /feeds', async () => {
    render(await FeedsSettingsPage())
    expect(screen.getByRole('link', { name: 'Back to feeds' })).toHaveAttribute('href', '/feeds')
  })
})
