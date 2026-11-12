import { fireEvent, render, screen } from '@testing-library/react'
import AppSections from '@/components/AppSections'
import { usePathname, useRouter } from '@/hooks/navigation'

describe('AppSections', () => {
  it('marks the section for the current page and switches on tap', () => {
    vi.mocked(usePathname).mockReturnValue('/books/library')
    const push = vi.fn()
    vi.mocked(useRouter).mockReturnValue({ push, replace: vi.fn(), back: vi.fn() })
    render(<AppSections app="books" />)

    expect(screen.getByRole('tablist', { name: 'Books sections' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Library' })).toHaveAttribute('aria-selected', 'true')
    fireEvent.click(screen.getByRole('tab', { name: 'Library' }))
    expect(push).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('tab', { name: 'Stats' }))
    expect(push).toHaveBeenCalledWith('/dashboard/reading')
  })

  it('links out to another service with a full navigation, not the router', () => {
    vi.mocked(usePathname).mockReturnValue('/monitoring/connections')
    const push = vi.fn()
    vi.mocked(useRouter).mockReturnValue({ push, replace: vi.fn(), back: vi.fn() })
    const open = vi.spyOn(window, 'open').mockReturnValue(null)
    render(<AppSections app="monitoring" />)

    fireEvent.click(screen.getByRole('tab', { name: 'Grafana' }))
    expect(open).toHaveBeenCalledWith('/grafana', '_self')
    expect(push).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('tab', { name: 'Automated actions' }))
    expect(push).toHaveBeenCalledWith('/monitoring/observability')
    expect(open).toHaveBeenCalledTimes(1)
    open.mockRestore()
  })

  it('sits in its own block, apart from whatever follows, so two tab tracks never share a line', () => {
    vi.mocked(usePathname).mockReturnValue('/books/library')
    render(<AppSections app="books" />)
    const wrapper = screen.getByRole('tablist').parentElement
    expect(wrapper).toHaveClass('mb-4')
    expect(wrapper?.tagName).toBe('DIV')
  })

  it('prefers the longest matching section and rewrites hrefs', () => {
    vi.mocked(usePathname).mockReturnValue('/feeds/stats')
    const push = vi.fn()
    vi.mocked(useRouter).mockReturnValue({ push, replace: vi.fn(), back: vi.fn() })
    render(<AppSections app="feeds" hrefFor={(href) => `${href}?x=1`} />)

    expect(screen.getByRole('tab', { name: 'Stats' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: 'Inbox' })).toHaveAttribute('aria-selected', 'false')
    fireEvent.click(screen.getByRole('tab', { name: 'Inbox' }))
    expect(push).toHaveBeenCalledWith('/feeds?x=1')
  })

  it('keeps a section selected on its sub-pages', () => {
    vi.mocked(usePathname).mockReturnValue('/books/library/extra')
    render(<AppSections app="books" />)
    expect(screen.getByRole('tab', { name: 'Library' })).toHaveAttribute('aria-selected', 'true')
  })

  it('offers every section when the page is outside them', () => {
    vi.mocked(usePathname).mockReturnValue('/games/settings')
    const push = vi.fn()
    vi.mocked(useRouter).mockReturnValue({ push, replace: vi.fn(), back: vi.fn() })
    render(<AppSections app="games" />)
    fireEvent.click(screen.getByRole('tab', { name: 'Library' }))
    expect(push).toHaveBeenCalledWith('/games/library')
  })

  it('renders no tabs for an app without sections', () => {
    render(<AppSections app="nope" />)
    expect(screen.getByRole('tablist', { name: 'nope sections' })).toBeInTheDocument()
    expect(screen.queryByRole('tab')).not.toBeInTheDocument()
  })

  it('selects nothing on a page outside its sections', () => {
    vi.mocked(usePathname).mockReturnValue('/books/settings')
    render(<AppSections app="books" />)
    for (const tab of screen.getAllByRole('tab')) {
      expect(tab).toHaveAttribute('aria-selected', 'false')
    }
  })
})
