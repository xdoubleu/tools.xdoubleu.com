import { render, screen } from '@testing-library/react'
import { Bar, BarChart } from 'recharts'
import { ChartCard, RankedList, StatTile, StatTileGrid } from '@/components/ui/stat'

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
    expect(screen.getByText('9')).toHaveClass('text-fg', 'text-2xl', 'font-semibold')
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

describe('StatTileGrid columns', () => {
  it('is four up from sm by default and three on request', () => {
    const { container, rerender } = render(<StatTileGrid>x</StatTileGrid>)
    expect(container.firstChild).toHaveClass('grid-cols-2', 'sm:grid-cols-4')
    rerender(<StatTileGrid columns={3}>x</StatTileGrid>)
    expect(container.firstChild).toHaveClass('grid-cols-2', 'sm:grid-cols-3')
    expect(container.firstChild).not.toHaveClass('sm:grid-cols-4')
  })
})

describe('ChartCard', () => {
  it('titles a chart and adds its footnote', () => {
    render(
      <ChartCard title="Per month" note="Undated items aren’t charted.">
        <BarChart data={[{ n: 1 }]}>
          <Bar dataKey="n" />
        </BarChart>
      </ChartCard>
    )
    expect(screen.getByText('Per month')).toBeInTheDocument()
    expect(screen.getByText('Undated items aren’t charted.')).toBeInTheDocument()
  })

  it('shows the empty message instead of the chart and its footnote', () => {
    render(
      <ChartCard title="Per month" empty="Nothing yet." note="A footnote">
        <BarChart data={[]} />
      </ChartCard>
    )
    expect(screen.getByText('Nothing yet.')).toBeInTheDocument()
    expect(screen.queryByText('A footnote')).not.toBeInTheDocument()
  })
})

describe('RankedList', () => {
  it('lists label/value rows with an optional sub line, named by its title', () => {
    render(
      <RankedList
        title="Top genres"
        empty="None"
        note="Counts titles"
        items={[
          { key: 'a', label: 'Drama', value: 3, sub: <em>tag</em> },
          { key: 'b', label: 'Comedy', value: 1 }
        ]}
      />
    )
    const rows = screen.getAllByRole('listitem')
    expect(screen.getByRole('list', { name: 'Top genres' })).toBeInTheDocument()
    expect(rows[0]).toHaveTextContent('Drama')
    expect(rows[0]).toHaveTextContent('tag')
    expect(rows[0]).toHaveTextContent('3')
    expect(rows[1]).not.toHaveTextContent('tag')
    expect(rows[1].querySelector('span.flex')).toBeNull()
    expect(screen.getByText('Counts titles')).toBeInTheDocument()
  })

  it('shows the empty message and no footnote without items', () => {
    render(<RankedList title="Top genres" empty="No genres yet." note="Counts titles" items={[]} />)
    expect(screen.getByText('No genres yet.')).toBeInTheDocument()
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
    expect(screen.queryByText('Counts titles')).not.toBeInTheDocument()
  })
})
