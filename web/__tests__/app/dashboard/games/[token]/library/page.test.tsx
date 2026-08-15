import React from 'react'
import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  GetSharedSteamResponseSchema,
  type GetSharedSteamResponse
} from '@/lib/gen/dashboard/v1/games_pb'

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/dashboard/GamesDashboardPublicLibrary', () => () => (
  <div data-testid="profile-games-library" />
))

const mockFetchOrNull = jest.fn<Promise<GetSharedSteamResponse | null>, [unknown]>(async () => null)

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

import GamesDashboardPublicLibraryPage, {
  metadata
} from '@/app/dashboard/games/[token]/library/page'

describe('GamesDashboardPublicLibraryPage', () => {
  beforeEach(() => jest.clearAllMocks())

  it('renders the Library heading and the library component', async () => {
    mockFetchOrNull.mockResolvedValue(null)
    render(await GamesDashboardPublicLibraryPage({ params: Promise.resolve({ token: 'tok-1' }) }))
    expect(screen.getByRole('heading', { name: 'Library' })).toBeInTheDocument()
    expect(screen.getByTestId('profile-games-library')).toBeInTheDocument()
  })

  it('renders a generic breadcrumb back to the dashboard when the link is invalid', async () => {
    mockFetchOrNull.mockResolvedValue(null)
    render(await GamesDashboardPublicLibraryPage({ params: Promise.resolve({ token: 'tok-1' }) }))
    expect(screen.getByRole('link', { name: 'Games' })).toHaveAttribute(
      'href',
      '/dashboard/games/tok-1'
    )
  })

  it("uses the owner's display name in the breadcrumb", async () => {
    mockFetchOrNull.mockResolvedValue(create(GetSharedSteamResponseSchema, { displayName: 'Bob' }))
    render(await GamesDashboardPublicLibraryPage({ params: Promise.resolve({ token: 'tok-1' }) }))
    expect(screen.getByRole('link', { name: "Bob's games" })).toHaveAttribute(
      'href',
      '/dashboard/games/tok-1'
    )
  })

  it('is excluded from search indexing', () => {
    expect(metadata.robots).toEqual({ index: false, follow: false })
  })
})
