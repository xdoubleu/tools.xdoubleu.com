import React from 'react'
import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  GetSharedLibraryResponseSchema,
  type GetSharedLibraryResponse
} from '@/lib/gen/reading/v1/public_pb'

jest.mock('@/components/profile/ProfileBooksClient', () => () => (
  <div data-testid="profile-books" />
))

const mockFetchOrNull = jest.fn<Promise<GetSharedLibraryResponse | null>, [unknown]>(
  async () => null
)

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

import ProfileBooksPage, { metadata } from '@/app/profile/reading/[token]/page'

describe('ProfileBooksPage', () => {
  beforeEach(() => jest.clearAllMocks())

  it('renders a generic heading and client component when the link is invalid', async () => {
    mockFetchOrNull.mockResolvedValue(null)
    render(await ProfileBooksPage({ params: Promise.resolve({ token: 'tok-1' }) }))
    expect(screen.getByRole('heading', { name: 'Shared books' })).toBeInTheDocument()
    expect(screen.getByTestId('profile-books')).toBeInTheDocument()
  })

  it("renders the owner's display name in the heading", async () => {
    mockFetchOrNull.mockResolvedValue(
      create(GetSharedLibraryResponseSchema, { displayName: 'Alice' })
    )
    render(await ProfileBooksPage({ params: Promise.resolve({ token: 'tok-1' }) }))
    expect(screen.getByRole('heading', { name: "Alice's books" })).toBeInTheDocument()
  })

  it('is excluded from search indexing', () => {
    expect(metadata.robots).toEqual({ index: false, follow: false })
  })
})
