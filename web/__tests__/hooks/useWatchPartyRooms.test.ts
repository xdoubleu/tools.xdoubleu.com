import { renderHook } from '@testing-library/react'

const mockClient = vi.hoisted(() => ({ createRoom: vi.fn(), joinRoom: vi.fn() }))
vi.mock('@/lib/client', () => ({ createServiceClient: vi.fn(() => mockClient) }))
vi.mock('@/lib/gen/watchparty/v1/rooms_pb', () => ({ RoomService: {} }))

import { useCreateRoom, useJoinRoom } from '@/hooks/useWatchPartyRooms'

beforeEach(() => vi.clearAllMocks())

describe('useCreateRoom', () => {
  it('resolves to the new room code', async () => {
    mockClient.createRoom.mockResolvedValue({ room: { roomCode: 'ABC' } })
    const { result } = renderHook(() => useCreateRoom())
    expect(await result.current()).toBe('ABC')
    expect(mockClient.createRoom).toHaveBeenCalledWith({})
  })

  it('resolves to an empty code when the response has no room', async () => {
    mockClient.createRoom.mockResolvedValue({})
    const { result } = renderHook(() => useCreateRoom())
    expect(await result.current()).toBe('')
  })
})

describe('useJoinRoom', () => {
  it('trims the code and resolves to the joined room code', async () => {
    mockClient.joinRoom.mockResolvedValue({ room: { roomCode: 'XYZ' } })
    const { result } = renderHook(() => useJoinRoom())
    expect(await result.current('  xyz ')).toBe('XYZ')
    expect(mockClient.joinRoom).toHaveBeenCalledWith({ roomCode: 'xyz' })
  })

  it('resolves to an empty code when the response has no room', async () => {
    mockClient.joinRoom.mockResolvedValue({})
    const { result } = renderHook(() => useJoinRoom())
    expect(await result.current('xyz')).toBe('')
  })
})
