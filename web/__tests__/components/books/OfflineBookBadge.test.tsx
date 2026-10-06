import { render, screen } from '@testing-library/react'
import OfflineBookBadge from '@/components/books/OfflineBookBadge'

jest.mock('@/hooks/useOfflineBooks', () => ({
  useStoredBookIds: () => new Set(['stored'])
}))

describe('OfflineBookBadge', () => {
  it('marks a book stored on this device', () => {
    render(<OfflineBookBadge bookId="stored" />)
    expect(screen.getByText('Available offline')).toBeInTheDocument()
  })

  it('renders nothing for a book that is not stored', () => {
    const { container } = render(<OfflineBookBadge bookId="other" />)
    expect(container).toBeEmptyDOMElement()
  })
})
