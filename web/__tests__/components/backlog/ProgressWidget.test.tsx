import React from 'react'
import { render, screen } from '@testing-library/react'
import ProgressWidget from '@/components/backlog/ProgressWidget'

jest.mock('@/lib/backlog/progressWebSocket', () => ({
  useProgressWebSocket: jest.fn()
}))

import { useProgressWebSocket } from '@/lib/backlog/progressWebSocket'

const mockUseProgressWebSocket = useProgressWebSocket as jest.Mock

describe('ProgressWidget', () => {
  it('shows Disconnected when status is not OPEN', () => {
    mockUseProgressWebSocket.mockReturnValue({ status: WebSocket.CLOSED, lastMessage: null })
    render(<ProgressWidget wsUrl="ws://localhost/ws" />)
    expect(screen.getByText('Disconnected')).toBeInTheDocument()
  })

  it('shows Connected when status is OPEN', () => {
    mockUseProgressWebSocket.mockReturnValue({ status: WebSocket.OPEN, lastMessage: null })
    render(<ProgressWidget wsUrl="ws://localhost/ws" />)
    expect(screen.getByText('Connected')).toBeInTheDocument()
  })

  it('renders lastMessage when provided', () => {
    mockUseProgressWebSocket.mockReturnValue({
      status: WebSocket.OPEN,
      lastMessage: 'Sync complete'
    })
    render(<ProgressWidget wsUrl="ws://localhost/ws" />)
    expect(screen.getByText('Sync complete')).toBeInTheDocument()
  })
})
