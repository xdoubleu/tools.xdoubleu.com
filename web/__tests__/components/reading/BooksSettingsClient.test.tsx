import React from 'react'
import { render, screen } from '@testing-library/react'

const mockImportBooks = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useImportBooks: () => mockImportBooks
}))

jest.mock('@/hooks/useBookFeeds', () => ({
  useFeeds: () => ({ data: { feeds: [] }, error: undefined, isLoading: false }),
  useCreateFeed: () => jest.fn(),
  useUpdateFeed: () => jest.fn(),
  useDeleteFeed: () => jest.fn(),
  useRefreshFeed: () => jest.fn()
}))

jest.mock('@/hooks/useAuth', () => ({
  useCurrentUser: jest.fn()
}))

jest.mock('@/components/reading/BulkBookUploader', () => ({
  __esModule: true,
  default: () => <div data-testid="bulk-uploader" />
}))

jest.mock('@/components/reading/KoboSetup', () => ({
  __esModule: true,
  default: () => <div data-testid="kobo-setup" />
}))

jest.mock('@/components/reading/KoboDevices', () => ({
  __esModule: true,
  default: () => <div data-testid="kobo-devices" />
}))

jest.mock('swr', () => ({ __esModule: true, mutate: jest.fn(), default: jest.fn() }))

import { useCurrentUser } from '@/hooks/useAuth'
import BooksSettingsClient from '@/components/reading/BooksSettingsClient'

const mockUseCurrentUser = jest.mocked(useCurrentUser)

function renderAsAdmin() {
  // @ts-expect-error -- partial mock
  mockUseCurrentUser.mockReturnValue({ data: { role: 'admin' }, isLoading: false })
  return render(<BooksSettingsClient />)
}

function renderAsUser() {
  // @ts-expect-error -- partial mock
  mockUseCurrentUser.mockReturnValue({ data: { role: 'user' }, isLoading: false })
  return render(<BooksSettingsClient />)
}

describe('BooksSettingsClient', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    // Default: non-admin user
    // @ts-expect-error -- partial mock
    mockUseCurrentUser.mockReturnValue({ data: { role: 'user' }, isLoading: false })
  })

  it('renders the Reading Settings heading', () => {
    render(<BooksSettingsClient />)
    expect(screen.getByRole('heading', { name: 'Reading Settings' })).toBeInTheDocument()
  })

  it('renders a breadcrumb link back to /reading', () => {
    render(<BooksSettingsClient />)
    expect(screen.getByRole('link', { name: 'Reading' })).toHaveAttribute('href', '/reading')
  })

  it('renders the Import books section', () => {
    render(<BooksSettingsClient />)
    expect(screen.getByText('Import books')).toBeInTheDocument()
    expect(screen.getByText('Import CSV')).toBeInTheDocument()
  })

  it('renders the Upload ebooks section with BulkBookUploader', () => {
    render(<BooksSettingsClient />)
    expect(screen.getByText('Upload ebooks')).toBeInTheDocument()
    expect(screen.getByTestId('bulk-uploader')).toBeInTheDocument()
  })

  it('renders the Kobo section header', () => {
    render(<BooksSettingsClient />)
    expect(screen.getByText('Kobo')).toBeInTheDocument()
  })

  it('renders the KoboSetup component', () => {
    render(<BooksSettingsClient />)
    expect(screen.getByTestId('kobo-setup')).toBeInTheDocument()
  })

  it('renders the KoboDevices component', () => {
    render(<BooksSettingsClient />)
    expect(screen.getByTestId('kobo-devices')).toBeInTheDocument()
  })

  it('renders the Connected devices heading', () => {
    render(<BooksSettingsClient />)
    expect(screen.getByText('Connected devices')).toBeInTheDocument()
  })

  it('does not show resync or find-duplicates on the settings page', () => {
    render(<BooksSettingsClient />)
    expect(screen.queryByTestId('resync-books-btn')).not.toBeInTheDocument()
    expect(screen.queryByTestId('find-duplicates-btn')).not.toBeInTheDocument()
  })

  it('shows Admin tools section with link for admin users', () => {
    renderAsAdmin()
    expect(screen.getByText('Admin tools')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open admin tools' })).toHaveAttribute(
      'href',
      '/reading/admin'
    )
  })

  it('hides Admin tools section for non-admin users', () => {
    renderAsUser()
    expect(screen.queryByText('Admin tools')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Open admin tools' })).not.toBeInTheDocument()
  })
})
