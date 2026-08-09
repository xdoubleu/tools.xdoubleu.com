import { fireEvent, render, screen } from '@testing-library/react'
import { AppearanceSection } from '@/components/settings/AppearanceSection'
import { THEME_KEY } from '@/lib/theme'

beforeEach(() => {
  localStorage.clear()
  document.documentElement.removeAttribute('data-theme')
  Object.defineProperty(window, 'matchMedia', {
    value: () => ({ matches: false }),
    writable: true
  })
})

describe('AppearanceSection', () => {
  it('selects auto when nothing is stored', () => {
    render(<AppearanceSection />)
    expect(screen.getByLabelText('Auto (match system)')).toBeChecked()
  })

  it('selects the stored theme', () => {
    localStorage.setItem(THEME_KEY, 'dark')
    render(<AppearanceSection />)
    expect(screen.getByLabelText('Dark')).toBeChecked()
  })

  it('stores and applies the picked theme', () => {
    render(<AppearanceSection />)
    fireEvent.click(screen.getByLabelText('Dark'))

    expect(localStorage.getItem(THEME_KEY)).toBe('dark')
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(screen.getByLabelText('Dark')).toBeChecked()
  })
})
