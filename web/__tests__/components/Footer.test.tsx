import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import Footer from '@/components/Footer'

jest.mock('@/lib/env', () => ({
  getRelease: jest.fn()
}))

jest.mock('@/components/BugReportModal', () => {
  return function MockBugReportModal({
    isOpen,
    onClose
  }: {
    isOpen: boolean
    onClose: () => void
  }) {
    if (!isOpen) return null
    return (
      <div data-testid="bug-report-modal">
        <button onClick={onClose}>Close Modal</button>
      </div>
    )
  }
})

import { getRelease } from '@/lib/env'

const mockGetRelease = getRelease as jest.Mock

beforeEach(() => {
  jest.clearAllMocks()
  mockGetRelease.mockReturnValue('')
})

describe('Footer', () => {
  it('renders copyright with current year', async () => {
    mockGetRelease.mockReturnValue('')

    render(<Footer />)

    const year = new Date().getFullYear()
    await waitFor(() => {
      expect(screen.getByText(new RegExp(`© ${year}`))).toBeInTheDocument()
    })
  })

  it('renders link to xdoubleu.com', async () => {
    mockGetRelease.mockReturnValue('')

    render(<Footer />)

    await waitFor(() => {
      const link = screen.getByRole('link', { name: /xdoubleu\.com/ })
      expect(link).toBeInTheDocument()
      expect(link).toHaveAttribute('href', 'https://xdoubleu.com')
    })
  })

  it('renders Report a bug button', async () => {
    mockGetRelease.mockReturnValue('')

    render(<Footer />)

    await waitFor(() => {
      const bugButton = screen.getByRole('button', { name: /Report a bug/ })
      expect(bugButton).toBeInTheDocument()
    })
  })

  it('opens bug report modal when button is clicked', async () => {
    mockGetRelease.mockReturnValue('')

    render(<Footer />)

    const bugButton = screen.getByRole('button', { name: /Report a bug/ })
    fireEvent.click(bugButton)

    await waitFor(() => {
      expect(screen.getByTestId('bug-report-modal')).toBeInTheDocument()
    })
  })

  it('renders release hash when available', async () => {
    mockGetRelease.mockReturnValue('abc123def456')

    render(<Footer />)

    await waitFor(() => {
      expect(screen.getByText('abc123d')).toBeInTheDocument()
    })
  })

  it('truncates release hash to 7 characters', async () => {
    mockGetRelease.mockReturnValue('abc123def456ghi789')

    render(<Footer />)

    await waitFor(() => {
      expect(screen.getByText('abc123d')).toBeInTheDocument()
      expect(screen.queryByText('abc123def456ghi789')).not.toBeInTheDocument()
    })
  })

  it('does not render release span when release is empty', async () => {
    mockGetRelease.mockReturnValue('')

    render(<Footer />)

    await waitFor(() => {
      expect(screen.getByText(/Report a bug/)).toBeInTheDocument()
    })

    // Verify no hash is displayed
    expect(screen.queryByText(/^[a-f0-9]{7}$/)).not.toBeInTheDocument()
  })

  it('renders all footer elements together', async () => {
    mockGetRelease.mockReturnValue('abc1234567')

    const { container } = render(<Footer />)

    await waitFor(() => {
      const footer = container.querySelector('footer')
      expect(footer).toBeInTheDocument()
      expect(footer).toHaveClass('border-t', 'border-border', 'bg-card')
    })

    // Check all elements are present
    const year = new Date().getFullYear()
    expect(screen.getByText(new RegExp(`© ${year}`))).toBeInTheDocument()
    expect(screen.getByText('abc1234')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Report a bug/ })).toBeInTheDocument()
  })

  it('calls getRelease on component mount', async () => {
    mockGetRelease.mockReturnValue('xyz789')

    render(<Footer />)

    await waitFor(() => {
      expect(mockGetRelease).toHaveBeenCalled()
    })
  })

  it('uses responsive Tailwind classes for layout', async () => {
    mockGetRelease.mockReturnValue('abc1234')

    const { container } = render(<Footer />)

    const footer = container.querySelector('footer')
    expect(footer).toHaveClass('px-4', 'py-3', 'sm:px-6')

    const divWrapper = footer?.querySelector('div')
    expect(divWrapper).toHaveClass(
      'flex',
      'flex-wrap',
      'items-center',
      'justify-center',
      'gap-3',
      'sm:gap-4',
      'md:gap-6'
    )
  })

  it('displays release with monospace font', async () => {
    mockGetRelease.mockReturnValue('abc1234567')

    const { container } = render(<Footer />)

    await waitFor(() => {
      const releaseElement = container.querySelector('.font-mono')
      expect(releaseElement).toBeInTheDocument()
      expect(releaseElement).toHaveTextContent('abc1234')
    })
  })
})
