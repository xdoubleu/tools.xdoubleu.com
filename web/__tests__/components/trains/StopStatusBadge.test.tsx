import { render, screen } from '@testing-library/react'
import StopStatusBadge from '@/components/trains/StopStatusBadge'

describe('StopStatusBadge', () => {
  it('renders on-time', () => {
    render(<StopStatusBadge status="on_time" delaySeconds={0} />)
    expect(screen.getByText('On time')).toBeInTheDocument()
  })

  it('renders delayed with minutes, never as on-time', () => {
    render(<StopStatusBadge status="delayed" delaySeconds={185} />)
    expect(screen.getByText('Delayed 3m')).toBeInTheDocument()
    expect(screen.queryByText('On time')).not.toBeInTheDocument()
  })

  it('renders no live data for unknown, distinct from on-time', () => {
    render(<StopStatusBadge status="unknown" delaySeconds={0} />)
    expect(screen.getByText('No live data')).toBeInTheDocument()
  })

  it('renders skipped', () => {
    render(<StopStatusBadge status="skipped" delaySeconds={0} />)
    expect(screen.getByText('Skipped')).toBeInTheDocument()
  })

  it('renders cancelled', () => {
    render(<StopStatusBadge status="cancelled" delaySeconds={0} />)
    expect(screen.getByText('Cancelled')).toBeInTheDocument()
  })

  it('falls back to no live data for an unrecognized status', () => {
    render(<StopStatusBadge status="something-new" delaySeconds={0} />)
    expect(screen.getByText('No live data')).toBeInTheDocument()
  })
})
