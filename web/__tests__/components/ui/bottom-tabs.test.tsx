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
    expect(alpha).toHaveClass('bg-fg')
    expect(screen.getByRole('link', { name: 'Beta' })).not.toHaveAttribute('aria-current')
  })

  it('publishes its height as --bottom-offset and clears it on unmount', () => {
    const height = vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockReturnValue(56)
    const { unmount } = render(<BottomTabs tabs={tabs} aria-label="Sections" />)
    expect(document.documentElement.style.getPropertyValue('--bottom-offset')).toBe('56px')
    unmount()
    expect(document.documentElement.style.getPropertyValue('--bottom-offset')).toBe('')
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
