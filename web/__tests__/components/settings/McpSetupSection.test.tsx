import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { McpSetupSection } from '@/components/settings/McpSetupSection'

describe('McpSetupSection', () => {
  it('shows only the Apps MCP command', () => {
    render(<McpSetupSection />)

    expect(screen.getByText('Apps MCP')).toBeInTheDocument()
    expect(screen.queryByText('Monitoring MCP')).not.toBeInTheDocument()
  })

  it('copies the apps MCP command to the clipboard', async () => {
    const writeText = jest.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })

    render(<McpSetupSection />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Copy' })[0])

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(
        'claude mcp add --transport http tools-apps https://tools.xdoubleu.com/api/apps/mcp'
      )
    })
    expect(await screen.findByRole('button', { name: 'Copied!' })).toBeInTheDocument()
  })
})
