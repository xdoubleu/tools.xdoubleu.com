import { render, screen } from '@testing-library/react'
import { StatTile, StatTileGrid } from '@/components/ui/stat'

describe('StatTile', () => {
  it('renders its label and value', () => {
    render(<StatTile label="Errors" value={12} />)
    expect(screen.getByText('Errors')).toBeInTheDocument()
    expect(screen.getByText('12')).toBeInTheDocument()
  })

  it('renders a hint when given', () => {
    render(<StatTile label="Latency" value="120ms" hint="p95" />)
    expect(screen.getByText('p95')).toBeInTheDocument()
  })

  it('omits the hint when not given', () => {
    const { container } = render(<StatTile label="Latency" value="120ms" />)
    expect(container.querySelectorAll('p')).toHaveLength(2)
  })

  it('colours the value by tone, leaving the label muted', () => {
    render(<StatTile label="Failures" value="3" tone="danger" />)
    expect(screen.getByText('3')).toHaveClass('text-danger')
    expect(screen.getByText('Failures')).toHaveClass('text-muted')
  })

  it('defaults to the neutral tone', () => {
    render(<StatTile label="Total" value="9" />)
    expect(screen.getByText('9')).toHaveClass('text-fg')
  })

  it('renders as a link when href is given', () => {
    render(<StatTile label="Open" value="4" href="/monitoring" />)
    expect(screen.getByRole('link')).toHaveAttribute('href', '/monitoring')
  })

  it('renders no link when href is omitted', () => {
    render(<StatTile label="Open" value="4" />)
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })

  it('merges a className override onto the link variant', () => {
    render(<StatTile label="Open" value="4" href="/x" className="custom-tile" />)
    expect(screen.getByRole('link')).toHaveClass('custom-tile')
  })

  it('merges a className override onto the static variant', () => {
    const { container } = render(<StatTile label="Open" value="4" className="custom-tile" />)
    expect(container.firstChild).toHaveClass('custom-tile')
  })
})

describe('StatTileGrid', () => {
  it('renders its children in a grid', () => {
    const { container } = render(
      <StatTileGrid>
        <StatTile label="A" value="1" />
        <StatTile label="B" value="2" />
      </StatTileGrid>
    )
    expect(container.firstChild).toHaveClass('grid')
    expect(screen.getByText('A')).toBeInTheDocument()
    expect(screen.getByText('B')).toBeInTheDocument()
  })

  it('merges a className override', () => {
    const { container } = render(
      <StatTileGrid className="sm:grid-cols-2">
        <StatTile label="A" value="1" />
      </StatTileGrid>
    )
    expect(container.firstChild).toHaveClass('sm:grid-cols-2')
  })
})
