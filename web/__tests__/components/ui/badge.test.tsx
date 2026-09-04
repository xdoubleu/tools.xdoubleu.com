import { render, screen } from '@testing-library/react'
import { Badge } from '@/components/ui/badge'

describe('Badge', () => {
  it('renders its children', () => {
    render(<Badge>EPUB</Badge>)
    expect(screen.getByText('EPUB')).toBeInTheDocument()
  })

  it('uses the accent treatment by default', () => {
    render(<Badge>EPUB</Badge>)
    expect(screen.getByText('EPUB')).toHaveClass('text-accent')
  })

  it.each([
    ['secondary', 'text-subtle'],
    ['success', 'text-success'],
    ['warn', 'text-warn'],
    ['danger', 'text-danger']
  ] as const)('applies the %s variant', (variant, expected) => {
    render(<Badge variant={variant}>Status</Badge>)
    expect(screen.getByText('Status')).toHaveClass(expected)
  })

  it('lets a className override win over the variant default', () => {
    render(<Badge className="text-fg">EPUB</Badge>)
    const badge = screen.getByText('EPUB')
    expect(badge).toHaveClass('text-fg')
    expect(badge).not.toHaveClass('text-accent')
  })

  it('forwards arbitrary span attributes', () => {
    render(<Badge data-testid="format-badge">EPUB</Badge>)
    expect(screen.getByTestId('format-badge')).toBeInTheDocument()
  })
})
