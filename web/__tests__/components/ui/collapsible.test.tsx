import { render, screen, fireEvent } from '@testing-library/react'
import { Collapsible } from '@/components/ui/collapsible'

describe('Collapsible', () => {
  it('starts collapsed by default', () => {
    render(
      <Collapsible title="Details">
        <p>hidden body</p>
      </Collapsible>
    )
    expect(screen.queryByText('hidden body')).not.toBeInTheDocument()
    expect(screen.getByRole('button')).toHaveAttribute('aria-expanded', 'false')
  })

  it('starts expanded when defaultCollapsed is false', () => {
    render(
      <Collapsible title="Details" defaultCollapsed={false}>
        <p>visible body</p>
      </Collapsible>
    )
    expect(screen.getByText('visible body')).toBeInTheDocument()
    expect(screen.getByRole('button')).toHaveAttribute('aria-expanded', 'true')
  })

  it('toggles its content on click', () => {
    render(
      <Collapsible title="Details">
        <p>body</p>
      </Collapsible>
    )

    fireEvent.click(screen.getByRole('button'))
    expect(screen.getByText('body')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button'))
    expect(screen.queryByText('body')).not.toBeInTheDocument()
  })

  it('renders its title on the trigger', () => {
    render(
      <Collapsible title="Details">
        <p>body</p>
      </Collapsible>
    )
    expect(screen.getByRole('button', { name: /Details/ })).toBeInTheDocument()
  })

  it('merges className and triggerClassName overrides', () => {
    const { container } = render(
      <Collapsible title="Details" className="custom-wrap" triggerClassName="text-sm">
        <p>body</p>
      </Collapsible>
    )
    expect(container.firstChild).toHaveClass('custom-wrap')
    expect(screen.getByRole('button')).toHaveClass('text-sm')
  })
})
