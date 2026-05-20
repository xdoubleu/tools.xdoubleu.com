import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import Footer from '@/components/Footer'

jest.mock('@/lib/env', () => ({
  getRelease: jest.fn(),
  getApiUrl: jest.fn(() => 'http://localhost:4000')
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
  global.fetch = jest.fn().mockResolvedValue({
    json: () => Promise.resolve({ release: '' })
  })
})

describe('Footer', () => {
  it('renders copyright with current year', async () => {
    render(<Footer />)

    const year = new Date().getFullYear()
    await waitFor(() => {
      expect(screen.getByText(new RegExp(`© ${year}`))).toBeInTheDocument()
    })
  })

  it('renders link to xdoubleu.com with text xdoubleu', async () => {
    render(<Footer />)

    await waitFor(() => {
      const link = screen.getByRole('link', { name: 'xdoubleu' })
      expect(link).toBeInTheDocument()
      expect(link).toHaveAttribute('href', 'https://xdoubleu.com')
    })
  })

  it('link has underline class', async () => {
    const { container } = render(<Footer />)

    await waitFor(() => {
      const link = container.querySelector('a[href="https://xdoubleu.com"]')
      expect(link).toHaveClass('underline')
    })
  })

  it('renders Report a bug button', async () => {
    render(<Footer />)

    await waitFor(() => {
      const bugButton = screen.getByRole('button', { name: /Report a bug/ })
      expect(bugButton).toBeInTheDocument()
    })
  })

  it('opens bug report modal when button is clicked', async () => {
    render(<Footer />)

    const bugButton = screen.getByRole('button', { name: /Report a bug/ })
    fireEvent.click(bugButton)

    await waitFor(() => {
      expect(screen.getByTestId('bug-report-modal')).toBeInTheDocument()
    })
  })

  it('renders web release hash when available', async () => {
    mockGetRelease.mockReturnValue('abc123def456')

    render(<Footer />)

    await waitFor(() => {
      expect(screen.getByText('web:abc123d')).toBeInTheDocument()
    })
  })

  it('renders api release hash fetched from server', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      json: () => Promise.resolve({ release: 'deadbeef1234' })
    })

    render(<Footer />)

    await waitFor(() => {
      expect(screen.getByText('api:deadbee')).toBeInTheDocument()
    })
  })

  it('renders both web and api hashes when both available', async () => {
    mockGetRelease.mockReturnValue('abc123def456')
    global.fetch = jest.fn().mockResolvedValue({
      json: () => Promise.resolve({ release: 'deadbeef1234' })
    })

    render(<Footer />)

    await waitFor(() => {
      expect(screen.getByText('web:abc123d')).toBeInTheDocument()
      expect(screen.getByText('api:deadbee')).toBeInTheDocument()
    })
  })

  it('truncates release hashes to 7 characters', async () => {
    mockGetRelease.mockReturnValue('abc123def456ghi789')

    render(<Footer />)

    await waitFor(() => {
      expect(screen.getByText('web:abc123d')).toBeInTheDocument()
      expect(screen.queryByText('abc123def456ghi789')).not.toBeInTheDocument()
    })
  })

  it('does not render version block when both releases are empty', async () => {
    mockGetRelease.mockReturnValue('')
    global.fetch = jest.fn().mockResolvedValue({
      json: () => Promise.resolve({ release: '' })
    })

    render(<Footer />)

    await waitFor(() => {
      expect(screen.getByText(/Report a bug/)).toBeInTheDocument()
    })

    expect(screen.queryByText(/^(web|api):/)).not.toBeInTheDocument()
  })

  it('handles fetch failure gracefully', async () => {
    mockGetRelease.mockReturnValue('abc123def456')
    global.fetch = jest.fn().mockRejectedValue(new Error('network error'))

    render(<Footer />)

    await waitFor(() => {
      expect(screen.getByText('web:abc123d')).toBeInTheDocument()
    })

    expect(screen.queryByText(/^api:/)).not.toBeInTheDocument()
  })

  it('renders all footer elements together', async () => {
    mockGetRelease.mockReturnValue('abc123def456')
    global.fetch = jest.fn().mockResolvedValue({
      json: () => Promise.resolve({ release: 'deadbeef1234' })
    })

    const { container } = render(<Footer />)

    await waitFor(() => {
      const footer = container.querySelector('footer')
      expect(footer).toBeInTheDocument()
      expect(footer).toHaveClass('border-t', 'border-border', 'bg-card')
      expect(screen.getByText('web:abc123d')).toBeInTheDocument()
      expect(screen.getByText('api:deadbee')).toBeInTheDocument()
    })

    const year = new Date().getFullYear()
    expect(screen.getByText(new RegExp(`© ${year}`))).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Report a bug/ })).toBeInTheDocument()
  })

  it('uses responsive Tailwind classes for layout', async () => {
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

  it('displays versions with monospace font', async () => {
    mockGetRelease.mockReturnValue('abc123def456')

    const { container } = render(<Footer />)

    await waitFor(() => {
      const monoElement = container.querySelector('.font-mono')
      expect(monoElement).toBeInTheDocument()
      expect(monoElement).toHaveTextContent('web:abc123d')
    })
  })

  it('fetches api version from correct endpoint', async () => {
    render(<Footer />)

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('http://localhost:4000/api/version')
    })
  })
})
