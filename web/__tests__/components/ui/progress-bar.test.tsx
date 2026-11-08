import { render, screen } from '@testing-library/react'
import { ProgressBar } from '@/components/ui/progress-bar'

describe('ProgressBar', () => {
  it('exposes its value as a progressbar', () => {
    render(<ProgressBar value={3} max={12} aria-label="Chapters" valueText="3 of 12" />)
    const bar = screen.getByRole('progressbar', { name: 'Chapters' })
    expect(bar).toHaveAttribute('aria-valuenow', '3')
    expect(bar).toHaveAttribute('aria-valuemax', '12')
    expect(bar).toHaveAttribute('aria-valuetext', '3 of 12')
    expect(bar.firstElementChild).toHaveStyle({ width: '25%' })
    expect(bar.firstElementChild).toHaveClass('bg-accent')
  })

  it('clamps out-of-range values', () => {
    const { rerender } = render(<ProgressBar value={150} aria-label="p" />)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '100')
    rerender(<ProgressBar value={-5} aria-label="p" />)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0')
  })

  it('renders empty for a zero max and takes a tone', () => {
    render(<ProgressBar value={0} max={0} tone="success" aria-label="p" />)
    const fill = screen.getByRole('progressbar').firstElementChild
    expect(fill).toHaveStyle({ width: '0%' })
    expect(fill).toHaveClass('bg-success')
  })
})
