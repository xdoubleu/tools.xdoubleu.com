import { render, screen, waitFor } from '@testing-library/react'
import Footer from '@/components/Footer'

jest.mock('@/lib/env', () => ({
  getRelease: jest.fn(),
  getApiUrl: jest.fn(() => 'https://api.example.com/api')
}))

import { getRelease } from '@/lib/env'

const mockGetRelease = jest.mocked(getRelease)
const mockFetch = jest.fn()

function jsonResponse(release: string, ok = true) {
  return { ok, json: () => Promise.resolve({ release }) }
}

beforeEach(() => {
  jest.clearAllMocks()
  mockGetRelease.mockReturnValue('')
  mockFetch.mockImplementation((url: string) => {
    if (url.endsWith('/api/version')) return Promise.resolve(jsonResponse(''))
    if (url === '/gateway/version') return Promise.resolve(jsonResponse(''))
    return Promise.reject(new Error(`unexpected fetch: ${url}`))
  })
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
  global.fetch = mockFetch as unknown as typeof fetch
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

  it('renders the web release hash, labeled and truncated to 7 characters', async () => {
    mockGetRelease.mockReturnValue('abc123def456ghi789')

    render(<Footer />)

    await waitFor(() => {
      expect(screen.getByText('web abc123d')).toBeInTheDocument()
      expect(screen.queryByText(/abc123def456ghi789/)).not.toBeInTheDocument()
    })
  })

  it('renders the api release fetched from the version endpoint', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === 'https://api.example.com/api/api/version') {
        return Promise.resolve(jsonResponse('def456abc123'))
      }
      return Promise.resolve(jsonResponse(''))
    })

    render(<Footer />)

    await waitFor(() => {
      expect(screen.getByText('api def456a')).toBeInTheDocument()
    })
  })

  it('renders the gateway release fetched from /gateway/version', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/gateway/version') return Promise.resolve(jsonResponse('789ghi012jkl'))
      return Promise.resolve(jsonResponse(''))
    })

    render(<Footer />)

    await waitFor(() => {
      expect(screen.getByText('gateway 789ghi0')).toBeInTheDocument()
    })
  })

  it('does not render a release badge when a release is empty', async () => {
    mockGetRelease.mockReturnValue('')

    render(<Footer />)

    await waitFor(() => {
      expect(screen.getByText(/xdoubleu/)).toBeInTheDocument()
    })

    expect(screen.queryByText(/^(web|api|gateway) /)).not.toBeInTheDocument()
  })

  it('does not render a badge when its fetch fails', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.endsWith('/api/version')) return Promise.reject(new Error('network error'))
      return Promise.resolve(jsonResponse(''))
    })

    render(<Footer />)

    await waitFor(() => {
      expect(screen.getByText(/xdoubleu/)).toBeInTheDocument()
    })

    expect(screen.queryByText(/^api /)).not.toBeInTheDocument()
  })

  it('uses responsive Tailwind classes for layout', async () => {
    const { container } = render(<Footer />)

    const footer = container.querySelector('footer')
    expect(footer).toHaveClass('px-4', 'py-3', 'sm:px-6')

    const divWrapper = footer?.querySelector('div')
    expect(divWrapper).toHaveClass(
      'mx-auto',
      'flex',
      'flex-wrap',
      'items-center',
      'justify-center',
      'gap-3',
      'sm:gap-4'
    )
  })

  it('displays the release hash with monospace font', async () => {
    mockGetRelease.mockReturnValue('abc123def456')

    const { container } = render(<Footer />)

    await waitFor(() => {
      const monoElement = container.querySelector('.font-mono')
      expect(monoElement).toBeInTheDocument()
      expect(monoElement).toHaveTextContent('web abc123d')
    })
  })
})
