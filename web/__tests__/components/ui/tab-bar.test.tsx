import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { Star } from 'lucide-react'
import { TabBar } from '@/components/ui/tab-bar'

const items = [
  { id: 'a', label: 'Alpha', href: '/alpha', icon: Star, active: true },
  { id: 'b', label: 'Beta', href: '/beta', icon: Star, active: false }
]

const renderBar = () =>
  render(
    <MemoryRouter>
      <TabBar items={items} aria-label="Primary" />
    </MemoryRouter>
  )

describe('TabBar', () => {
  it('renders a labelled landmark with one link per item', () => {
    renderBar()
    const nav = screen.getByRole('navigation', { name: 'Primary' })
    expect(nav).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Alpha' })).toHaveAttribute('href', '/alpha')
    expect(screen.getByRole('link', { name: 'Beta' })).toHaveAttribute('href', '/beta')
  })

  it('marks only the active item', () => {
    renderBar()
    expect(screen.getByRole('link', { name: 'Alpha' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Alpha' })).toHaveClass('text-accent')
    expect(screen.getByRole('link', { name: 'Beta' })).not.toHaveAttribute('aria-current')
    expect(screen.getByRole('link', { name: 'Beta' })).not.toHaveClass('text-accent')
  })

  it('is hidden from lg up and clears the home indicator', () => {
    renderBar()
    const nav = screen.getByRole('navigation')
    expect(nav).toHaveClass('lg:hidden', 'pb-[env(safe-area-inset-bottom)]')
  })
})
