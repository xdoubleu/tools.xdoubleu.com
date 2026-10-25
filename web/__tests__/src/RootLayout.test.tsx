import { act, render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router'
import RootLayout from '@/src/RootLayout'
import { usePathname } from '@/hooks/navigation'

vi.mock('@/components/AppShell', () => ({
  default: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="shell">{children}</div>
  )
}))
vi.mock('@/src/WebVitals', () => ({ WebVitals: () => <i data-testid="vitals" /> }))

// ScrollRestoration needs a data router, as in src/routes.tsx.
function mount() {
  const router = createMemoryRouter([
    {
      Component: RootLayout,
      children: [
        { index: true, element: <p>home page</p> },
        { path: 'detail', element: <p>detail page</p> }
      ]
    }
  ])
  render(<RouterProvider router={router} />)
  return router
}

function scrollWindowTo(y: number) {
  Object.defineProperty(window, 'scrollY', { value: y, configurable: true })
}

describe('RootLayout', () => {
  beforeEach(() => {
    document.head.innerHTML =
      '<link id="app-icon" rel="icon" href="/icon.svg"><link id="app-apple-icon" rel="apple-touch-icon" href="/apple-icon.png">'
    vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    scrollWindowTo(0)
  })

  it('renders the matched page inside the app shell', () => {
    vi.mocked(usePathname).mockReturnValue('/')
    mount()

    expect(screen.getByTestId('shell')).toHaveTextContent('home page')
    expect(screen.getByTestId('vitals')).toBeInTheDocument()
  })

  it('opens a new page at the top and restores the position on Back', async () => {
    const router = mount()
    expect(await screen.findByText('home page')).toBeInTheDocument()

    scrollWindowTo(640)
    await act(() => router.navigate('/detail'))
    expect(await screen.findByText('detail page')).toBeInTheDocument()
    expect(window.scrollTo).toHaveBeenLastCalledWith(0, 0)

    scrollWindowTo(0)
    await act(() => router.navigate(-1))
    expect(await screen.findByText('home page')).toBeInTheDocument()
    expect(window.scrollTo).toHaveBeenLastCalledWith(0, 640)
  })

  it("uses an app's own icons under its path", () => {
    vi.mocked(usePathname).mockReturnValue('/books/library')
    mount()

    expect(document.getElementById('app-icon')).toHaveAttribute('href', '/icons/books.svg')
    expect(document.getElementById('app-apple-icon')).toHaveAttribute(
      'href',
      '/icons/books-apple.png'
    )
  })

  it('uses the suite icons elsewhere', () => {
    vi.mocked(usePathname).mockReturnValue('/trains')
    mount()

    expect(document.getElementById('app-icon')).toHaveAttribute('href', '/icon.svg')
    expect(document.getElementById('app-apple-icon')).toHaveAttribute('href', '/apple-icon.png')
  })
})
