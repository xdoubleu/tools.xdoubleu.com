'use client'

import { useState } from 'react'
import { createServiceClient } from '@/lib/client'
import { RoomService } from '@/lib/gen/watchparty/v1/rooms_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PageContainer } from '@/components/ui/page-container'

export default function WatchpartyPage() {
  const [roomCode, setRoomCode] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const client = createServiceClient(RoomService)

  async function handleCreate() {
    setError(null)
    setLoading(true)
    try {
      const room = await client.createRoom({})
      if (room.room?.roomCode) {
        window.location.href = `/watchparty/${room.room.roomCode}/presenter`
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create room')
    } finally {
      setLoading(false)
    }
  }

  async function handleJoin(e: React.FormEvent) {
    e.preventDefault()
    if (!roomCode.trim()) return
    setError(null)
    setLoading(true)
    try {
      const room = await client.joinRoom({ roomCode: roomCode.trim() })
      if (room.room?.roomCode) {
        window.location.href = `/watchparty/${room.room.roomCode}`
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to join room')
    } finally {
      setLoading(false)
    }
  }

  return (
    <PageContainer className="max-w-md p-6 mt-16">
      <h1 className="text-3xl font-bold mb-8 text-center">Watch Party</h1>

      <div className="mb-8">
        <Button size="lg" className="w-full" onClick={handleCreate} disabled={loading}>
          {loading ? 'Creating…' : 'Create Room'}
        </Button>
      </div>

      <div className="relative mb-8">
        <div className="absolute inset-0 flex items-center">
          <div className="w-full border-t border-border" />
        </div>
        <div className="relative flex justify-center text-sm">
          <span className="bg-bg px-2 text-muted">or join a room</span>
        </div>
      </div>

      <form onSubmit={handleJoin} className="flex gap-2">
        <Input
          type="text"
          value={roomCode}
          onChange={(e) => setRoomCode(e.target.value)}
          placeholder="Room code"
          className="flex-1"
        />
        <Button type="submit" variant="secondary" disabled={loading || !roomCode.trim()}>
          Join
        </Button>
      </form>

      {error && <p className="mt-4 text-center text-sm text-danger">{error}</p>}
    </PageContainer>
  )
}
