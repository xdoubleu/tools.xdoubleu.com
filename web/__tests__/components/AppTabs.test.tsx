import { render, screen } from '@testing-library/react'
import AppTabs from '@/components/AppTabs'
import { usePathname } from '@/hooks/navigation'

function at(pathname: string) {
  vi.mocked(usePathname).mockReturnValue(pathname)
  return render(<AppTabs />)
}

describe('AppTabs', () => {
  it.each(['/', '/trains', '/books/42/read', '/dashboard/reading/some-token', '/recipes'])(
    'renders nothing on %s',
    (path) => {
      const { container } = at(path)
      expect(container).toBeEmptyDOMElement()
    }
  )

  it('marks Library active on a book page', () => {
    at('/books/42')
    expect(screen.getByRole('navigation', { name: 'Books sections' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Library' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Settings' })).not.toHaveAttribute('aria-current')
  })

  it('marks Settings active, not Library, on books settings', () => {
    at('/books/settings')
    expect(screen.getByRole('link', { name: 'Settings' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Library' })).not.toHaveAttribute('aria-current')
  })

  it('switches between the dashboard sections', () => {
    at('/dashboard/games')
    expect(screen.getByRole('link', { name: 'Games' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Reading' })).toHaveAttribute(
      'href',
      '/dashboard/reading'
    )
  })
})
