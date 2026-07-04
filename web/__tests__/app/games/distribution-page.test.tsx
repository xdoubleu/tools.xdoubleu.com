import { render, screen } from '@testing-library/react'

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

jest.mock(
  '@/app/games/distribution/[bucket]/SteamDistributionClient',
  () => (props: { bucket: string }) => <div data-testid="distribution-client">{props.bucket}</div>
)

import Page from '@/app/games/distribution/[bucket]/page'

describe('SteamDistributionPage', () => {
  it('awaits params and renders the client with the bucket', async () => {
    render(await Page({ params: Promise.resolve({ bucket: '80' }) }))
    expect(screen.getByTestId('distribution-client')).toHaveTextContent('80')
  })
})
