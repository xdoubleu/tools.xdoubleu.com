import { render, screen } from '@testing-library/react'

jest.mock('next/link', () => ({
  __esModule: true,
  default: ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  )
}))

import { Breadcrumb } from '@/components/ui/breadcrumb'

describe('Breadcrumb', () => {
  it('renders linked ancestors and a non-linked current page', () => {
    render(
      <Breadcrumb
        items={[
          { label: 'Backlog', href: '/backlog' },
          { label: 'Games', href: '/backlog/steam' },
          { label: 'The Witcher 3' }
        ]}
      />
    )

    expect(screen.getByRole('link', { name: 'Backlog' })).toHaveAttribute('href', '/backlog')
    expect(screen.getByRole('link', { name: 'Games' })).toHaveAttribute('href', '/backlog/steam')

    const current = screen.getByText('The Witcher 3')
    expect(current.tagName).toBe('SPAN')
    expect(current).toHaveAttribute('aria-current', 'page')
  })

  it('renders the last item as plain text even when it has an href', () => {
    render(<Breadcrumb items={[{ label: 'Recipes', href: '/recipes/list' }]} />)

    expect(screen.queryByRole('link', { name: 'Recipes' })).not.toBeInTheDocument()
    expect(screen.getByText('Recipes')).toHaveAttribute('aria-current', 'page')
  })

  it('renders a separator between items but not before the first', () => {
    render(<Breadcrumb items={[{ label: 'Backlog', href: '/backlog' }, { label: 'Settings' }]} />)

    expect(screen.getAllByText('/')).toHaveLength(1)
  })

  it('merges a custom className onto the nav', () => {
    render(<Breadcrumb className="mb-6" items={[{ label: 'Home' }]} />)

    expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toHaveClass('mb-6')
  })
})
