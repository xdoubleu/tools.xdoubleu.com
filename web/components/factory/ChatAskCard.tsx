import { useState, type FormEvent } from 'react'
import type { ChatItem } from '@/lib/factory/chat'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { SectionHeading } from '@/components/ui/section-heading'
import { TogglePill } from '@/components/ui/toggle-pill'

type Ask = Extract<ChatItem, { type: 'ask' }>

interface ChatAskCardProps {
  item: Ask
  /** False once the chat is closed: an unanswered card can no longer be answered. */
  open: boolean
  onAnswer: (askId: number, text: string) => Promise<void>
}

/** A question the agent asked: its options, "other" with free text, then the answer. */
export default function ChatAskCard({ item, open, onAnswer }: ChatAskCardProps) {
  const [chosen, setChosen] = useState<string[]>([])
  const [other, setOther] = useState('')
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)
  const answered = item.answer !== null
  const disabled = !open || answered || pending

  const send = async (text: string) => {
    setPending(true)
    setFailed(false)
    try {
      await onAnswer(item.seq, text)
    } catch {
      setFailed(true)
    }
    setPending(false)
  }

  const pick = (option: string) => {
    if (!item.multi) return void send(option)
    setChosen((c) => (c.includes(option) ? c.filter((o) => o !== option) : [...c, option]))
  }

  const submitOther = (e: FormEvent) => {
    e.preventDefault()
    const text = other.trim()
    if (text) void send(text)
  }

  return (
    <Card variant="inset" padding="md" className="space-y-3" data-testid="chat-ask">
      <SectionHeading level="eyebrow">Question</SectionHeading>
      <p className="break-words text-sm font-medium">{item.question}</p>
      {answered ? (
        <p className="break-words text-sm">
          <span className="text-muted">Answered: </span>
          {item.answer}
        </p>
      ) : (
        <>
          <div className="flex flex-wrap gap-2" role="group" aria-label="Options">
            {item.options.map((option) =>
              item.multi ? (
                <TogglePill
                  key={option}
                  label={option}
                  active={chosen.includes(option)}
                  disabled={disabled}
                  onClick={() => pick(option)}
                />
              ) : (
                <Button
                  key={option}
                  type="button"
                  variant="secondary"
                  className="h-auto min-h-11 whitespace-normal py-2 text-left"
                  disabled={disabled}
                  onClick={() => pick(option)}
                >
                  {option}
                </Button>
              )
            )}
          </div>
          {item.multi && (
            <Button
              type="button"
              disabled={disabled || chosen.length === 0}
              onClick={() => void send(chosen.join(', '))}
            >
              Send choices
            </Button>
          )}
          <form onSubmit={submitOther} className="flex gap-2">
            <Input
              aria-label="Other answer"
              placeholder="Other…"
              value={other}
              disabled={disabled}
              onChange={(e) => setOther(e.target.value)}
            />
            <Button
              type="submit"
              variant="secondary"
              aria-label="Send other answer"
              disabled={disabled || other.trim() === ''}
            >
              Send
            </Button>
          </form>
          {!open && <p className="text-sm text-muted">The chat closed before this was answered.</p>}
          {failed && <Alert tone="danger">That didn&apos;t go through. Try again.</Alert>}
        </>
      )}
    </Card>
  )
}
