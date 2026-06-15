import React from 'react'
import { render, screen } from '@testing-library/react'

const mockImportBooks = jest.fn()

jest.mock('@/hooks/useBacklog', () => ({
  useImportBooks: () => mockImportBooks,
  useClearLibrary: () => jest.fn(),
  useFindDuplicates: () => ({ data: undefined, isLoading: false, mutate: jest.fn() }),
  useMergeBooks: () => jest.fn()
}))

jest.mock('@/components/backlog/BulkBookUploader', () => ({
  __esModule: true,
  default: () => <div data-testid="bulk-uploader" />
}))

jest.mock('@/components/backlog/KoboSetup', () => ({
  __esModule: true,
  default: () => <div data-testid="kobo-setup" />
}))

jest.mock('@/components/backlog/KoboDevices', () => ({
  __esModule: true,
  default: () => <div data-testid="kobo-devices" />
}))

jest.mock('@/components/backlog/ClearLibraryDialog', () => ({
  __esModule: true,
  default: ({ open }: { open: boolean }) => (open ? <div data-testid="clear-dialog" /> : null)
}))

jest.mock('swr', () => ({ __esModule: true, mutate: jest.fn(), default: jest.fn() }))

import BacklogBooksSettingsPage from '@/app/backlog/books/settings/page'

describe('BacklogBooksSettingsPage', () => {
  it('renders the Books Settings heading', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByRole('heading', { name: 'Books Settings' })).toBeInTheDocument()
  })

  it('renders a breadcrumb link back to /backlog/books', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByRole('link', { name: 'Books' })).toHaveAttribute('href', '/backlog/books')
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
})
