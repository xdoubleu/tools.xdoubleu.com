import { useEffect, useRef, useState, type FormEvent } from 'react'
import type { Chat } from '@/lib/gen/factory/v1/factory_pb'
import { stateLabel } from '@/lib/factory/chat'
import { useChatSocket } from '@/hooks/useChatSocket'
import { useChatActions } from '@/hooks/useFactory'
import ChatTranscript from '@/components/factory/ChatTranscript'
import { Alert } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { ConfirmDialog } from '@/components/ui/dialog'
import { PageHeader } from '@/components/ui/page-header'
import { SectionHeading } from '@/components/ui/section-heading'
import { Textarea } from '@/components/ui/textarea'

interface ChatViewProps {
  id: string
  /** The chat's row from the list: its report once closed. */
  chat: Chat | undefined
  onLeave: () => void
}

/** One chat: the streamed transcript, a composer pinned above the keyboard, and Close. */
export default function ChatView({ id, chat, onLeave }: ChatViewProps) {
  const { transcript, state, connected } = useChatSocket(id)
  const { send, answer, close } = useChatActions()
  const [text, setText] = useState('')
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)
  const [closing, setClosing] = useState(false)
  const [report, setReport] = useState('')
  const bottom = useRef<HTMLDivElement>(null)

  const current = state || chat?.state || ''
  const stopped = current === 'stopped'
  const canSend = current === 'idle' || current === 'asking'
  const items = transcript.items.length

  useEffect(() => {
    bottom.current?.scrollIntoView?.({ block: 'end' })
  }, [items, transcript.items[items - 1]])

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const body = text.trim()
    if (!body || !canSend) return
    setPending(true)
    setFailed(false)
    try {
      await (current === 'asking' ? answer(id, body) : send(id, body))
      setText('')
    } catch {
      setFailed(true)
    }
    setPending(false)
  }

  const finish = async () => {
    setPending(true)
    setFailed(false)
    try {
      const stoppedChat = await close(id)
      setReport(stoppedChat?.report ?? '')
      setClosing(false)
    } catch {
      setFailed(true)
    }
    setPending(false)
  }

  const leftover = report || chat?.report || ''

  return (
    <>
      <PageHeader
        title="Chat"
        breadcrumb={[{ label: 'Chats', href: '/factory/chat' }, { label: 'Chat' }]}
        actions={
          <>
            <Badge variant={stopped ? 'secondary' : 'success'}>
              {current ? stateLabel(current) : 'Starting'}
            </Badge>
            {!stopped && (
              <Button variant="destructive" onClick={() => setClosing(true)}>
                Close chat
              </Button>
            )}
          </>
        }
      />
      {!connected && !stopped && (
        <Alert tone="warn" className="mb-3">
          Reconnecting…
        </Alert>
      )}
      {(current === '' || (current === 'idle' && items === 0)) && !stopped && (
        <p className="mb-3 text-sm text-muted" role="status">
          {current === '' ? 'Starting…' : 'Ready. Say what you want done.'}
        </p>
      )}
      <ChatTranscript items={transcript.items} />
      {stopped && (
        <Card variant="inset" padding="md" className="mt-4 space-y-2" data-testid="chat-report">
          <SectionHeading>Left behind</SectionHeading>
          <p className="break-words text-sm">{leftover || 'No report was recorded.'}</p>
          <Button variant="secondary" onClick={onLeave}>
            Back to chats
          </Button>
        </Card>
      )}
      <div ref={bottom} />
      {!stopped && (
        <form
          onSubmit={(e) => void submit(e)}
          className="sticky bottom-[calc(var(--tab-bar-height)+env(safe-area-inset-bottom))] mt-4 space-y-2 bg-bg py-2 lg:bottom-0"
        >
          {failed && <Alert tone="danger">That didn&apos;t go through. Try again.</Alert>}
          <Textarea
            aria-label={current === 'asking' ? 'Your answer' : 'Message'}
            rows={2}
            value={text}
            disabled={!canSend || pending}
            placeholder={canSend ? 'Message the agent' : stateLabel(current)}
            onChange={(e) => setText(e.target.value)}
          />
          <Button type="submit" disabled={!canSend || pending || text.trim() === ''}>
            {current === 'asking' ? 'Answer' : 'Send'}
          </Button>
        </form>
      )}
      <ConfirmDialog
        open={closing}
        onOpenChange={setClosing}
        title="Close this chat?"
        description="The agent stops. Anything it left uncommitted or unpushed is reported afterwards."
        confirmLabel="Close chat"
        pendingLabel="Closing…"
        destructive
        pending={pending}
        onConfirm={() => void finish()}
      />
    </>
  )
}
