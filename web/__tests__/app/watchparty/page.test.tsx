import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    createRoom: jest.fn(),
    joinRoom: jest.fn()
  }))
}))
jest.mock('@/lib/gen/watchparty/v1/rooms_pb', () => ({ RoomService: {} }))

import WatchpartyPage from '@/app/watchparty/page'

describe('WatchpartyPage', () => {
  it('renders the title and room actions', () => {
    render(<WatchpartyPage />)
    expect(screen.getByRole('heading', { name: 'Watch Party' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Create Room' })).toBeInTheDocument()
  })
})
