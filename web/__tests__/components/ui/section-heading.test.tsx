import { render, screen } from '@testing-library/react'
import { SectionHeading } from '@/components/ui/section-heading'

describe('SectionHeading', () => {
  it('renders a serif h2 by default', () => {
    render(<SectionHeading>Shelves</SectionHeading>)
    const heading = screen.getByRole('heading', { level: 2, name: 'Shelves' })
    expect(heading).toHaveClass('font-display', 'text-xl')
  })

  it('renders the eyebrow as a small uppercase h3', () => {
    render(<SectionHeading level="eyebrow">Tags</SectionHeading>)
    const heading = screen.getByRole('heading', { level: 3, name: 'Tags' })
    expect(heading).toHaveClass('uppercase', 'text-xs', 'text-muted')
  })

  it('honours as and applies className to the heading without actions', () => {
    render(
      <SectionHeading as="h4" id="h" className="mb-2">
        Notes
      </SectionHeading>
    )
    const heading = screen.getByRole('heading', { level: 4, name: 'Notes' })
    expect(heading).toHaveAttribute('id', 'h')
    expect(heading).toHaveClass('mb-2')
  })

  it('puts actions on the heading row', () => {
    const { container } = render(
      <SectionHeading actions={<button>Add</button>} className="mb-3">
        Feeds
      </SectionHeading>
    )
    expect(container.firstElementChild).toHaveClass('flex', 'mb-3')
    expect(screen.getByRole('heading', { name: 'Feeds' })).not.toHaveClass('mb-3')
    expect(screen.getByRole('button', { name: 'Add' })).toBeInTheDocument()
  })
})
