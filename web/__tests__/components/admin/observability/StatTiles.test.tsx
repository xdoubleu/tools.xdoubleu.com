import { render, screen } from '@testing-library/react'
import StatTiles from '@/components/admin/observability/StatTiles'

describe('StatTiles', () => {
  it('renders labels and values with danger tone', () => {
    render(
      <StatTiles
        tiles={[
          { label: 'R2 storage', value: '1.2 GB' },
          { label: 'Orphaned', value: '4.0 MB', tone: 'danger' }
        ]}
      />
    )

    expect(screen.getByText('R2 storage')).toBeInTheDocument()
    expect(screen.getByText('1.2 GB')).toBeInTheDocument()
    const orphaned = screen.getByText('4.0 MB')
    expect(orphaned).toHaveClass('text-danger')
  })
})
