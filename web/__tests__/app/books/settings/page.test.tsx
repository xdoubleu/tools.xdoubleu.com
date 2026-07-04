import React from 'react'
import { render, screen } from '@testing-library/react'

const mockImportBooks = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useImportBooks: () => mockImportBooks,
  useCompareCSV: () => jest.fn(),
  useClearLibrary: () => jest.fn()
}))

jest.mock('@/hooks/useAuth', () => ({
  useCurrentUser: jest.fn()
}))

jest.mock('@/components/books/BulkBookUploader', () => ({
  __esModule: true,
  default: () => <div data-testid="bulk-uploader" />
}))

jest.mock('@/components/books/KoboSetup', () => ({
  __esModule: true,
  default: () => <div data-testid="kobo-setup" />
}))

jest.mock('@/components/books/KoboDevices', () => ({
  __esModule: true,
  default: () => <div data-testid="kobo-devices" />
}))

jest.mock('@/components/books/ClearLibraryDialog', () => ({
  __esModule: true,
  default: ({ open }: { open: boolean }) => (open ? <div data-testid="clear-dialog" /> : null)
}))

jest.mock('@/components/books/CompareReport', () => ({
  __esModule: true,
  default: () => <div data-testid="compare-report" />
}))

jest.mock('swr', () => ({ __esModule: true, mutate: jest.fn(), default: jest.fn() }))

import { useCurrentUser } from '@/hooks/useAuth'
import BacklogBooksSettingsPage from '@/app/books/settings/page'

const mockUseCurrentUser = jest.mocked(useCurrentUser)

function renderAsAdmin() {
  // @ts-expect-error -- partial mock
  mockUseCurrentUser.mockReturnValue({ data: { role: 'admin' }, isLoading: false })
  return render(<BacklogBooksSettingsPage />)
}

function renderAsUser() {
  // @ts-expect-error -- partial mock
  mockUseCurrentUser.mockReturnValue({ data: { role: 'user' }, isLoading: false })
  return render(<BacklogBooksSettingsPage />)
}

describe('BacklogBooksSettingsPage', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    // Default: non-admin user
    // @ts-expect-error -- partial mock
    mockUseCurrentUser.mockReturnValue({ data: { role: 'user' }, isLoading: false })
  })

  it('renders the Books Settings heading', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByRole('heading', { name: 'Books Settings' })).toBeInTheDocument()
  })

  it('renders a breadcrumb link back to /books', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByRole('link', { name: 'Books' })).toHaveAttribute('href', '/books')
  })

  it('renders the Import books section', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByText('Import books')).toBeInTheDocument()
    expect(screen.getByText('Import CSV')).toBeInTheDocument()
  })

  it('renders the Upload ebooks section with BulkBookUploader', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByText('Upload ebooks')).toBeInTheDocument()
    expect(screen.getByTestId('bulk-uploader')).toBeInTheDocument()
  })

  it('renders the Kobo section header', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByText('Kobo')).toBeInTheDocument()
  })

  it('renders the KoboSetup component', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByTestId('kobo-setup')).toBeInTheDocument()
  })

  it('renders the KoboDevices component', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByTestId('kobo-devices')).toBeInTheDocument()
  })

  it('renders the Connected devices heading', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByText('Connected devices')).toBeInTheDocument()
  })

  it('renders the Danger zone section with clear-library button', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByText('Danger zone')).toBeInTheDocument()
    expect(screen.getByTestId('clear-library-btn')).toBeInTheDocument()
  })

  it('does not show resync or find-duplicates on the settings page', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.queryByTestId('resync-openlibrary-btn')).not.toBeInTheDocument()
    expect(screen.queryByTestId('find-duplicates-btn')).not.toBeInTheDocument()
  })

  it('shows Admin tools section with link for admin users', () => {
    renderAsAdmin()
    expect(screen.getByText('Admin tools')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open admin tools' })).toHaveAttribute(
      'href',
      '/books/admin'
    )
  })

  it('hides Admin tools section for non-admin users', () => {
    renderAsUser()
    expect(screen.queryByText('Admin tools')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Open admin tools' })).not.toBeInTheDocument()
  })
})
