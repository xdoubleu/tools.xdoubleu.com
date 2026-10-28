import { useState } from 'react'
import { useRouter } from '@/hooks/navigation'
import { useCreateRoom, useJoinRoom } from '@/hooks/useWatchPartyRooms'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { Alert } from '@/components/ui/alert'

export default function WatchpartyPage() {
  const [roomCode, setRoomCode] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const router = useRouter()
  const createRoom = useCreateRoom()
  const joinRoom = useJoinRoom()

  async function handleCreate() {
    setError(null)
    setLoading(true)
    try {
      const code = await createRoom()
      if (code) router.push(`/watchparty/${code}/presenter`)
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
      const code = await joinRoom(roomCode)
      if (code) router.push(`/watchparty/${code}`)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to join room')
    } finally {
      setLoading(false)
    }
  }

  return (
    <PageContainer size="narrow">
      <PageHeader title="Watch Party" />

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
          aria-label="Room code"
          placeholder="Room code"
          className="min-w-0 flex-1"
        />
        <Button type="submit" variant="secondary" disabled={loading || !roomCode.trim()}>
          Join
        </Button>
      </form>

      {error && (
        <Alert tone="danger" className="mt-4">
          {error}
        </Alert>
      )}
    </PageContainer>
  )
}
