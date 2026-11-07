import { render, screen } from '@testing-library/react'
import { BottomTabs } from '@/components/ui/bottom-tabs'

const tabs = [
  { href: '/a', label: 'Alpha', icon: <svg data-testid="icon" />, active: true },
  { href: '/b', label: 'Beta', icon: null, active: false }
]

describe('BottomTabs', () => {
  it('marks the active tab as the current page with a filled pill', () => {
    render(<BottomTabs tabs={tabs} aria-label="Sections" />)
    expect(screen.getByRole('navigation', { name: 'Sections' })).toHaveClass('sm:hidden')
    const alpha = screen.getByRole('link', { name: 'Alpha' })
    expect(alpha).toHaveAttribute('aria-current', 'page')
    expect(alpha).toHaveClass('bg-fg', 'text-bg', 'min-h-11', 'focus-visible:ring-2')
    expect(alpha.className).toContain('duration-press')
    const beta = screen.getByRole('link', { name: 'Beta' })
    expect(beta).not.toHaveAttribute('aria-current')
    expect(beta).toHaveClass('text-muted')
    expect(beta).not.toHaveClass('bg-fg')
  })

  it('publishes its height as --bottom-offset and clears it on unmount', () => {
    const height = vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockReturnValue(56)
    const { unmount } = render(<BottomTabs tabs={tabs} aria-label="Sections" />)
    const root = document.documentElement.style
    expect(root.getPropertyValue('--bottom-offset')).toBe('56px')
    expect(root.getPropertyValue('--bottom-safe')).toBe('0px')
    unmount()
    expect(root.getPropertyValue('--bottom-offset')).toBe('')
    expect(root.getPropertyValue('--bottom-safe')).toBe('')
    height.mockRestore()
  })

  it('follows resizes when ResizeObserver exists', () => {
    let notify = () => {}
    const disconnect = vi.fn()
    vi.stubGlobal(
      'ResizeObserver',
      class {
        constructor(cb: () => void) {
          notify = cb
        }
        observe() {}
        disconnect = disconnect
      }
    )
    const height = vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockReturnValue(56)
    const { unmount } = render(<BottomTabs tabs={tabs} aria-label="Sections" />)
    height.mockReturnValue(80)
    notify()
    expect(document.documentElement.style.getPropertyValue('--bottom-offset')).toBe('80px')
    unmount()
    expect(disconnect).toHaveBeenCalled()
    height.mockRestore()
    vi.unstubAllGlobals()
  })
})
