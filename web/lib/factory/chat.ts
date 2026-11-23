/** One frame of a chat socket: an event with its seq, or a state frame (seq 0). */
export interface ChatFrame {
  seq: number
  kind: string
  payload: string
  createdAt: string
  state?: string
}

interface PlanEntry {
  content: string
  status: string
}

export type ChatItem =
  | { type: 'user'; seq: number; text: string; answer: boolean }
  | { type: 'agent'; seq: number; text: string }
  | {
      type: 'tool'
      seq: number
      id: string
      title: string
      kind: string
      status: string
      detail: string
    }
  | {
      type: 'ask'
      /** The question's id: the seq of its event. */
      seq: number
      question: string
      options: string[]
      multi: boolean
      /** The person's answer, once given. */
      answer: string | null
    }
  | { type: 'plan'; seq: number; entries: PlanEntry[] }
  | { type: 'error'; seq: number; text: string }

export interface ChatTranscript {
  items: ChatItem[]
  /** The newest seq folded in; reconnect with it as `after`. */
  cursor: number
}

export const EMPTY_TRANSCRIPT: ChatTranscript = { items: [], cursor: 0 }

type Json = Record<string, unknown>

function isObj(v: unknown): v is Json {
  return v !== null && typeof v === 'object' && !Array.isArray(v)
}

function parse(payload: string): Json {
  try {
    const value: unknown = JSON.parse(payload)
    return isObj(value) ? value : {}
  } catch {
    return {}
  }
}

const str = (v: unknown): string => (typeof v === 'string' ? v : '')

function chunkText(update: Json): string {
  return isObj(update.content) ? str(update.content.text) : ''
}

/** What a tool call acted on: the first location, else its raw input's path or command. */
function target(update: Json): string {
  const first: unknown = Array.isArray(update.locations) ? update.locations[0] : undefined
  if (isObj(first) && str(first.path)) return str(first.path)
  const input = update.rawInput
  if (isObj(input)) {
    return str(input.filePath) || str(input.path) || str(input.command) || str(input.pattern)
  }
  return ''
}

function detailOf(update: Json): string {
  const input = update.rawInput
  const parts: string[] = []
  if (isObj(input) && Object.keys(input).length > 0) parts.push(JSON.stringify(input, null, 2))
  const output = update.rawOutput
  if (typeof output === 'string' && output) parts.push(output)
  return parts.join('\n\n')
}

function planEntries(update: Json): PlanEntry[] {
  const entries: unknown[] = Array.isArray(update.entries) ? update.entries : []
  return entries.filter(isObj).map((e) => ({
    content: str(e.content),
    status: str(e.status) || 'pending'
  }))
}

function applyUpdate(items: ChatItem[], seq: number, update: Json): ChatItem[] {
  switch (str(update.sessionUpdate)) {
    case 'agent_message_chunk': {
      const text = chunkText(update)
      const last = items[items.length - 1]
      if (last?.type === 'agent') {
        return [...items.slice(0, -1), { ...last, text: last.text + text }]
      }
      return [...items, { type: 'agent', seq, text }]
    }
    case 'tool_call':
      return [
        ...items,
        {
          type: 'tool',
          seq,
          id: str(update.toolCallId),
          title: str(update.title) || 'Tool call',
          kind: str(update.kind),
          status: str(update.status) || 'pending',
          detail: detailOf(update)
            ? `${target(update)}\n${detailOf(update)}`.trim()
            : target(update)
        }
      ]
    case 'tool_call_update': {
      const id = str(update.toolCallId)
      return items.map((item) => {
        if (item.type !== 'tool' || item.id !== id) return item
        const more = detailOf(update)
        return {
          ...item,
          title: str(update.title) || item.title,
          status: str(update.status) || item.status,
          detail: more ? `${item.detail}\n\n${more}`.trim() : item.detail
        }
      })
    }
    case 'plan': {
      const entries = planEntries(update)
      const at = items.findIndex((i) => i.type === 'plan')
      // A plan update replaces the whole plan, so one checklist stays in place.
      if (at >= 0) return items.map((item, i) => (i === at ? { type: 'plan', seq, entries } : item))
      return [...items, { type: 'plan', seq, entries }]
    }
    default:
      return items
  }
}

/** The tool line shown collapsed: its name and what it acted on. */
export function toolLine(item: Extract<ChatItem, { type: 'tool' }>): string {
  const t = item.detail.split('\n')[0] ?? ''
  return t && !item.title.includes(t) && !t.startsWith('{') ? `${item.title} · ${t}` : item.title
}

/** Folds one frame in. A seq already seen is dropped: a push may repeat a replay. */
export function applyFrame(t: ChatTranscript, frame: ChatFrame): ChatTranscript {
  if (frame.seq <= t.cursor) return t
  const cursor = frame.seq
  switch (frame.kind) {
    case 'user_message':
    case 'user_answer':
      return {
        cursor,
        items: [
          ...t.items,
          {
            type: 'user',
            seq: frame.seq,
            text: frame.payload,
            answer: frame.kind === 'user_answer'
          }
        ]
      }
    case 'ask': {
      const q = parse(frame.payload)
      const options = Array.isArray(q.options) ? q.options.filter((o) => typeof o === 'string') : []
      const ask: ChatItem = {
        type: 'ask',
        seq: frame.seq,
        question: str(q.question),
        options,
        multi: q.multi === true,
        answer: null
      }
      return { cursor, items: [...t.items, ask] }
    }
    case 'ask_answer': {
      const a = parse(frame.payload)
      const answer = str(a.answer)
      return {
        cursor,
        items: t.items.map((item) =>
          item.type === 'ask' && item.seq === a.ask ? { ...item, answer } : item
        )
      }
    }
    case 'update':
      return { cursor, items: applyUpdate(t.items, frame.seq, parse(frame.payload)) }
    case 'error':
      return { cursor, items: [...t.items, { type: 'error', seq: frame.seq, text: frame.payload }] }
    default:
      return { ...t, cursor }
  }
}

export function stateLabel(state: string): string {
  switch (state) {
    case 'working':
      return 'Working'
    case 'asking':
      return 'Waiting for your answer'
    case 'idle':
      return 'Ready'
    case 'paused':
      return 'Paused'
    case 'stopped':
      return 'Closed'
    default:
      return 'Starting'
  }
}

/** The socket URL of a chat, resuming after `after`. */
export function chatSocketUrl(apiUrl: string, id: string, after: number): string {
  const base = apiUrl
    .replace(/^https:\/\//, 'wss://')
    .replace(/^http:\/\//, 'ws://')
    .replace(/\/$/, '')
  return `${base}/factory/chats/${id}/ws?after=${after}`
}

export function isChatFrame(value: unknown): value is ChatFrame {
  return isObj(value) && typeof value.seq === 'number' && typeof value.kind === 'string'
}

/** The id of the question card still waiting for the person, if any. */
export function pendingAsk(items: ChatItem[]): number {
  const last = items.findLast((i) => i.type === 'ask' && i.answer === null)
  return last?.seq ?? 0
}
