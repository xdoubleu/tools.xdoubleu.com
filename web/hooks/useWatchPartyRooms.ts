import { useCallback, useMemo } from 'react'
import { createServiceClient } from '@/lib/client'
import { RoomService } from '@/lib/gen/watchparty/v1/rooms_pb'

/** Creates a room and resolves to its code ('' when the API returns none). */
export function useCreateRoom() {
  const client = useMemo(() => createServiceClient(RoomService), [])
  return useCallback(async () => (await client.createRoom({})).room?.roomCode ?? '', [client])
}

/** Joins a room by code and resolves to its code ('' when the API returns none). */
export function useJoinRoom() {
  const client = useMemo(() => createServiceClient(RoomService), [])
  return useCallback(
    async (roomCode: string) =>
      (await client.joinRoom({ roomCode: roomCode.trim() })).room?.roomCode ?? '',
    [client]
  )
}
