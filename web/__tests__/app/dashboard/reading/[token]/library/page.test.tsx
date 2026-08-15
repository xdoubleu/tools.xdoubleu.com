import React from 'react'
import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  GetSharedLibraryResponseSchema,
  type GetSharedLibraryResponse
} from '@/lib/gen/dashboard/v1/reading_pb'

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/dashboard/ReadingDashboardPublicLibrary', () => () => (
  <div data-testid="profile-books-library" />
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

import ReadingDashboardPublicLibraryPage, {
  metadata
} from '@/app/dashboard/reading/[token]/library/page'

describe('ReadingDashboardPublicLibraryPage', () => {
  beforeEach(() => jest.clearAllMocks())

  it('renders the Library heading and the library component', async () => {
    mockFetchOrNull.mockResolvedValue(null)
    render(await ReadingDashboardPublicLibraryPage({ params: Promise.resolve({ token: 'tok-1' }) }))
    expect(screen.getByRole('heading', { name: 'Library' })).toBeInTheDocument()
    expect(screen.getByTestId('profile-books-library')).toBeInTheDocument()
  })

  it('renders a generic breadcrumb back to the dashboard when the link is invalid', async () => {
    mockFetchOrNull.mockResolvedValue(null)
    render(await ReadingDashboardPublicLibraryPage({ params: Promise.resolve({ token: 'tok-1' }) }))
    expect(screen.getByRole('link', { name: 'Reading' })).toHaveAttribute(
      'href',
      '/dashboard/reading/tok-1'
    )
  })

  it("uses the owner's display name in the breadcrumb", async () => {
    mockFetchOrNull.mockResolvedValue(
      create(GetSharedLibraryResponseSchema, { displayName: 'Alice' })
    )
    render(await ReadingDashboardPublicLibraryPage({ params: Promise.resolve({ token: 'tok-1' }) }))
    expect(screen.getByRole('link', { name: "Alice's reading" })).toHaveAttribute(
      'href',
      '/dashboard/reading/tok-1'
    )
  })

  it('is excluded from search indexing', () => {
    expect(metadata.robots).toEqual({ index: false, follow: false })
  })
})
