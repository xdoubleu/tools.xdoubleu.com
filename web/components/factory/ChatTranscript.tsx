import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Check, Circle, CircleDot } from 'lucide-react'
import type { ChatItem } from '@/lib/factory/chat'
import { toolLine } from '@/lib/factory/chat'
import ChatAskCard from '@/components/factory/ChatAskCard'
import { Alert } from '@/components/ui/alert'
import { Card } from '@/components/ui/card'
import { Collapsible } from '@/components/ui/collapsible'
import { SectionHeading } from '@/components/ui/section-heading'

/** Markdown whose code blocks scroll inside their own box instead of widening the page. */
function Markdown({ text }: { text: string }) {
  return (
    <div className="prose prose-sm max-w-none break-words text-fg [&_pre]:overflow-x-auto">
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{text}</ReactMarkdown>
    </div>
  )
}

function PlanIcon({ status }: { status: string }) {
  if (status === 'completed') return <Check className="size-4 shrink-0 text-success" aria-hidden />
  if (status === 'in_progress') {
    return <CircleDot className="size-4 shrink-0 text-accent" aria-hidden />
  }
  return <Circle className="size-4 shrink-0 text-muted" aria-hidden />
}

interface AskProps {
  open: boolean
  onAnswer: (askId: number, text: string) => Promise<void>
}

function Item({ item, ask }: { item: ChatItem; ask: AskProps }) {
  switch (item.type) {
    case 'user':
      return (
        <Card variant="inset" padding="sm" className="ml-8 whitespace-pre-wrap break-words text-sm">
          {item.answer && <span className="text-muted">Answer: </span>}
          {item.text}
        </Card>
      )
    case 'agent':
      return <Markdown text={item.text} />
    case 'tool':
      return (
        <Collapsible title={toolLine(item)} triggerClassName="text-sm font-normal">
          <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-words rounded-xl bg-card p-3 text-xs">
            {`${item.status}\n${item.detail}`.trim()}
          </pre>
        </Collapsible>
      )
    case 'ask':
      return <ChatAskCard item={item} open={ask.open} onAnswer={ask.onAnswer} />
    case 'plan':
      return (
        <Card variant="inset" padding="sm" className="space-y-2">
          <SectionHeading level="eyebrow">Plan</SectionHeading>
          <ul className="space-y-1" aria-label="Plan">
            {item.entries.map((e, i) => (
              <li key={`${i}-${e.content}`} className="flex items-start gap-2 text-sm">
                <PlanIcon status={e.status} />
                <span className="min-w-0 break-words">{e.content}</span>
              </li>
            ))}
          </ul>
        </Card>
      )
    case 'error':
      return <Alert tone="danger">{item.text}</Alert>
  }
}

/** The streamed transcript of a chat. */
export default function ChatTranscript({ items, ...ask }: { items: ChatItem[] } & AskProps) {
  return (
    <div className="space-y-3" aria-live="polite" aria-label="Transcript" role="log">
      {items.map((item) => (
        <Item key={`${item.type}-${item.seq}`} item={item} ask={ask} />
      ))}
    </div>
  )
}
