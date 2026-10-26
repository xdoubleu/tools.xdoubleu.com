import { type ReactNode } from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { LinkCard } from '@/components/ui/link-card'

vi.mock('@/components/ui/link', () => ({
  Link: ({ href, children, ...rest }: { href: string; children?: ReactNode }) => (
    <a href={href} data-router="" {...rest}>
      {children}
    </a>
  ),
  useLinkPending: () => false
}))

describe('LinkCard', () => {
  it('navigates in-app by default', () => {
    render(<LinkCard href="/recipes">Recipes</LinkCard>)
    expect(screen.getByRole('link', { name: 'Recipes' })).toHaveAttribute('data-router')
  })

  it('loads the whole document for an external target', () => {
    render(
      <LinkCard href="/grafana" external linkClassName="p-4">
        Grafana
      </LinkCard>
    )
    const link = screen.getByRole('link', { name: 'Grafana' })
    expect(link).toHaveAttribute('href', '/grafana')
    expect(link).not.toHaveAttribute('data-router')
    expect(link).toHaveClass('p-4', 'block')
  })

  it('links only the content zone', () => {
    const onClick = vi.fn()
    render(
      <LinkCard href="/books/1" actions={<button onClick={onClick}>Update progress</button>}>
        <h3>Dune</h3>
      </LinkCard>
    )
    const link = screen.getByRole('link')
    expect(link).toHaveAttribute('href', '/books/1')
    expect(link).toHaveTextContent('Dune')
    const button = screen.getByRole('button', { name: 'Update progress' })
    expect(link).not.toContainElement(button)
    fireEvent.click(button)
    expect(onClick).toHaveBeenCalled()
  })

  it('renders no footer without actions', () => {
    const { container } = render(
      <LinkCard href="/x" aria-label="Open x">
        <span>x</span>
      </LinkCard>
    )
    expect(screen.getByRole('link', { name: 'Open x' })).toBeInTheDocument()
    expect(container.querySelector('.border-t')).toBeNull()
  })
})
