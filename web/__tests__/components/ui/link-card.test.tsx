import { fireEvent, render, screen } from '@testing-library/react'
import { LinkCard } from '@/components/ui/link-card'

jest.mock('next/link', () => {
  const Link = ({ children, href, ...rest }: { children: React.ReactNode; href: string }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  )
  return { __esModule: true, default: Link, useLinkStatus: () => ({ pending: false }) }
})

describe('LinkCard', () => {
  it('links only the content zone', () => {
    const onClick = jest.fn()
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
