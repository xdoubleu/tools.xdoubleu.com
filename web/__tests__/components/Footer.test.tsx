import { render, screen } from '@testing-library/react'
import Footer from '@/components/Footer'

vi.mock('@/lib/env', () => ({ getRelease: vi.fn() }))

import { getRelease } from '@/lib/env'

const mockGetRelease = vi.mocked(getRelease)

beforeEach(() => {
  vi.clearAllMocks()
  mockGetRelease.mockReturnValue('')
})

describe('Footer', () => {
  it('renders copyright with current year', () => {
    render(<Footer />)
    const year = new Date().getFullYear()
    expect(screen.getByText(new RegExp(`© ${year}`))).toBeInTheDocument()
  })

  it('renders an underlined link to xdoubleu.com', () => {
    render(<Footer />)
    const link = screen.getByRole('link', { name: 'xdoubleu' })
    expect(link).toHaveAttribute('href', 'https://xdoubleu.com')
    expect(link).toHaveClass('underline')
  })

  it('renders one release badge, truncated to 7 characters, without fetching', () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch')
    mockGetRelease.mockReturnValue('abc123def456ghi789')

    const { container } = render(<Footer />)

    const badges = container.querySelectorAll('.font-mono')
    expect(badges).toHaveLength(1)
    expect(badges[0]).toHaveTextContent(/^abc123d$/)
    expect(fetchSpy).not.toHaveBeenCalled()
    fetchSpy.mockRestore()
  })

  it('renders no badge when the release is empty', () => {
    const { container } = render(<Footer />)
    expect(container.querySelector('.font-mono')).not.toBeInTheDocument()
  })

  it('keeps clear of the home indicator and a landscape notch at every width', () => {
    const { container } = render(<Footer />)

    const footer = container.querySelector('footer')
    expect(footer).toHaveClass(
      'pl-[var(--inset-left)]',
      'pr-[var(--inset-right)]',
      'pb-[calc(0.25rem+env(safe-area-inset-bottom))]',
      'sm:pb-[calc(0.75rem+env(safe-area-inset-bottom))]'
    )
    expect(footer?.className).not.toMatch(/\b(sm|lg):(px|py)-/)
  })
})
