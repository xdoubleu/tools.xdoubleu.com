import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'

const { createRoom, joinRoom } = vi.hoisted(() => ({ createRoom: vi.fn(), joinRoom: vi.fn() }))
vi.mock('@/hooks/useWatchPartyRooms', () => ({
  useCreateRoom: () => createRoom,
  useJoinRoom: () => joinRoom
}))

import { useRouter } from '@/hooks/navigation'
import WatchpartyPage from '@/app/watchparty/page'

const push = vi.mocked(useRouter)().push

beforeEach(() => vi.clearAllMocks())

describe('WatchpartyPage', () => {
  it('renders the title and room actions', () => {
    render(<WatchpartyPage />)
    expect(screen.getByRole('heading', { name: 'Watch Party' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Create Room' })).toBeInTheDocument()
  })

  it('opens the presenter view of a created room', async () => {
    createRoom.mockResolvedValue('ABC')
    render(<WatchpartyPage />)
    fireEvent.click(screen.getByRole('button', { name: 'Create Room' }))
    await waitFor(() => expect(push).toHaveBeenCalledWith('/watchparty/ABC/presenter'))
  })

  it('opens a joined room', async () => {
    joinRoom.mockResolvedValue('XYZ')
    render(<WatchpartyPage />)
    fireEvent.change(screen.getByLabelText('Room code'), { target: { value: ' xyz ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Join' }))
    await waitFor(() => expect(push).toHaveBeenCalledWith('/watchparty/XYZ'))
    expect(joinRoom).toHaveBeenCalledWith(' xyz ')
  })

  it('stays put when no room code comes back', async () => {
    createRoom.mockResolvedValue('')
    joinRoom.mockResolvedValue('')
    render(<WatchpartyPage />)
    fireEvent.click(screen.getByRole('button', { name: 'Create Room' }))
    await waitFor(() => expect(createRoom).toHaveBeenCalled())
    fireEvent.change(screen.getByLabelText('Room code'), { target: { value: 'xyz' } })
    fireEvent.click(screen.getByRole('button', { name: 'Join' }))
    await waitFor(() => expect(joinRoom).toHaveBeenCalled())
    expect(push).not.toHaveBeenCalled()
  })

  it('shows the error when joining fails', async () => {
    joinRoom.mockRejectedValue(new Error('room not found'))
    render(<WatchpartyPage />)
    fireEvent.change(screen.getByLabelText('Room code'), { target: { value: 'nope' } })
    fireEvent.click(screen.getByRole('button', { name: 'Join' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('room not found')
    expect(push).not.toHaveBeenCalled()
  })
})
