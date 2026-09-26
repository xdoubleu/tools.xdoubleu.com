import React from 'react'
import { render, screen } from '@testing-library/react'
import { ConnectionRow } from '@/components/ui/connection-row'

describe('ConnectionRow', () => {
  it('shows a connected provider with its detail and actions', () => {
    render(
      <ConnectionRow
        name="Todoist"
        connected
        detail="Connected on 1 Jan"
        actions={<button>Disconnect</button>}
      />
    )
    expect(screen.getByText('Todoist')).toBeInTheDocument()
    expect(screen.getByText('Connected')).toBeInTheDocument()
    expect(screen.getByText('Connected on 1 Jan')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Disconnect' })).toBeInTheDocument()
  })

  it('shows a disconnected provider without detail or actions', () => {
    const { container } = render(<ConnectionRow name="Todoist" connected={false} />)
    expect(screen.getByText('Not connected')).toBeInTheDocument()
    expect(container.querySelectorAll('p')).toHaveLength(0)
  })
})
