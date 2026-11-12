import { useState } from 'react'
import { useRouter } from '@/hooks/navigation'
import { useCreateRoom, useJoinRoom } from '@/hooks/useWatchPartyRooms'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { Alert } from '@/components/ui/alert'
import { Card } from '@/components/ui/card'
import { SectionHeading } from '@/components/ui/section-heading'

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
      <PageHeader
        title="Watch party"
        description="Share a screen with someone and talk over it."
        actions={
          <Button onClick={handleCreate} disabled={loading}>
            {loading ? 'Creating…' : 'Create room'}
          </Button>
        }
      />

      <Card className="space-y-3">
        <SectionHeading level="eyebrow">Join a room</SectionHeading>
        <form onSubmit={handleJoin} className="flex gap-2">
          <Input
            type="text"
            value={roomCode}
            onChange={(e) => setRoomCode(e.target.value)}
            aria-label="Room code"
            placeholder="Room code"
            className="min-w-0 flex-1"
          />
          <Button type="submit" disabled={loading || !roomCode.trim()}>
            Join
          </Button>
        </form>
      </Card>

      {error && (
        <Alert tone="danger" className="mt-4">
          {error}
        </Alert>
      )}
    </PageContainer>
  )
}
