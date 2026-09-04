import { render, screen } from '@testing-library/react'
import { SectionCard } from '@/components/ui/section-card'

describe('SectionCard', () => {
  it('renders its title and children', () => {
    render(
      <SectionCard title="Host metrics">
        <p>body</p>
      </SectionCard>
    )
    expect(screen.getByRole('heading', { name: 'Host metrics' })).toBeInTheDocument()
    expect(screen.getByText('body')).toBeInTheDocument()
  })

  it('renders a description when given', () => {
    render(
      <SectionCard title="Host metrics" description="Last 24 hours">
        <p>body</p>
      </SectionCard>
    )
    expect(screen.getByText('Last 24 hours')).toBeInTheDocument()
  })

  it('renders an action when given', () => {
    render(
      <SectionCard title="Host metrics" action={<button type="button">Refresh</button>}>
        <p>body</p>
      </SectionCard>
    )
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeInTheDocument()
  })

  it('renders neither description nor action when omitted', () => {
    render(
      <SectionCard title="Host metrics">
        <p>body</p>
      </SectionCard>
    )
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Host metrics' })).toBeInTheDocument()
  })

  it('merges a className override onto the card', () => {
    const { container } = render(
      <SectionCard title="T" className="custom-card">
        <p>body</p>
      </SectionCard>
    )
    expect(container.firstChild).toHaveClass('custom-card')
  })

  it('applies contentClassName to the body, not the card', () => {
    const { container } = render(
      <SectionCard title="T" contentClassName="space-y-4">
        <p>body</p>
      </SectionCard>
    )
    expect(container.firstChild).not.toHaveClass('space-y-4')
    expect(container.querySelector('.space-y-4')).toBeInTheDocument()
  })
})
