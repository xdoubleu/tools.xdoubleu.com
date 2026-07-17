import React from 'react'
import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { GetSharedSteamResponseSchema } from '@/lib/gen/games/v1/public_pb'

jest.mock('@/components/profile/ProfileGamesClient', () => () => (
  <div data-testid="profile-games" />
))

const mockFetchOrNull = jest.fn<Promise<unknown>, [unknown]>(async () => null)

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: (fn: () => unknown) => mockFetchOrNull(fn)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

import ProfileGamesPage, { metadata } from '@/app/profile/games/[token]/page'

describe('ProfileGamesPage', () => {
  beforeEach(() => jest.clearAllMocks())

  it('renders a generic heading and client component when the link is invalid', async () => {
    mockFetchOrNull.mockResolvedValue(null)
    render(await ProfileGamesPage({ params: Promise.resolve({ token: 'tok-1' }) }))
    expect(screen.getByRole('heading', { name: 'Shared games' })).toBeInTheDocument()
    expect(screen.getByTestId('profile-games')).toBeInTheDocument()
  })

  it("renders the owner's display name in the heading", async () => {
    mockFetchOrNull.mockResolvedValue(create(GetSharedSteamResponseSchema, { displayName: 'Bob' }))
    render(await ProfileGamesPage({ params: Promise.resolve({ token: 'tok-1' }) }))
    expect(screen.getByRole('heading', { name: "Bob's games" })).toBeInTheDocument()
  })

  it('is excluded from search indexing', () => {
    expect(metadata.robots).toEqual({ index: false, follow: false })
  })
})
