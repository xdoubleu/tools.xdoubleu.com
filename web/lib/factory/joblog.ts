import type { JobEvent } from '@/lib/gen/factory/v1/factory_pb'

/** How often an open log is refreshed while its job is queued or running. */
export const LOG_POLL_MS = 2_000

/** What the log shows for one piece of the agent's output. */
export interface LogEntry {
  /** The event's seq plus the piece's index, so keys stay stable as events arrive. */
  key: string
  kind: 'text' | 'tool' | 'result' | 'error' | 'note'
  /** One line: who or what, e.g. "Bash" or "Result". */
  title: string
  body: string
}

const MAX_BODY = 4_000

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function str(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function clip(text: string): string {
  return text.length > MAX_BODY ? `${text.slice(0, MAX_BODY)}…` : text
}

/** A tool_result's content is a string or a list of {type:"text", text} blocks. */
function resultText(content: unknown): string {
  if (typeof content === 'string') return content
  if (!Array.isArray(content)) return ''
  return content
    .map((block: unknown) => (isRecord(block) ? str(block.text) : ''))
    .filter((text) => text !== '')
    .join('\n')
}

function toolSummary(input: unknown): string {
  if (!isRecord(input)) return ''
  // The field a person would look for first; otherwise the whole input.
  for (const field of ['command', 'file_path', 'path', 'pattern', 'url', 'description']) {
    const value = str(input[field])
    if (value !== '') return value
  }
  return Object.keys(input).length > 0 ? JSON.stringify(input) : ''
}

function entriesOf(seq: number, message: unknown, fromAgent: boolean): LogEntry[] {
  const content = isRecord(message) ? message.content : undefined
  if (typeof content === 'string') {
    return fromAgent && content !== ''
      ? [{ key: `${seq}.0`, kind: 'text', title: 'Agent', body: clip(content) }]
      : []
  }
  if (!Array.isArray(content)) return []

  const out: LogEntry[] = []
  content.forEach((block: unknown, i) => {
    if (!isRecord(block)) return
    const key = `${seq}.${i}`
    if (block.type === 'text' && fromAgent && str(block.text) !== '') {
      out.push({ key, kind: 'text', title: 'Agent', body: clip(str(block.text)) })
    } else if (block.type === 'tool_use') {
      out.push({
        key,
        kind: 'tool',
        title: str(block.name) || 'Tool',
        body: clip(toolSummary(block.input))
      })
    } else if (block.type === 'tool_result') {
      const text = resultText(block.content)
      out.push({
        key,
        kind: block.is_error === true ? 'error' : 'note',
        title: block.is_error === true ? 'Tool error' : 'Tool output',
        body: clip(text)
      })
    }
  })
  return out
}

/** The pieces of the log one event carries; events the log has no use for give none. */
export function logEntries(event: JobEvent): LogEntry[] {
  let parsed: unknown
  try {
    parsed = JSON.parse(event.payload)
  } catch {
    // A line cut at the size limit, or output that isn't JSON at all.
    return [{ key: `${event.seq}.0`, kind: 'note', title: 'Output', body: clip(event.payload) }]
  }
  if (!isRecord(parsed)) return []

  switch (parsed.type) {
    case 'assistant':
      return entriesOf(event.seq, parsed.message, true)
    case 'user':
      return entriesOf(event.seq, parsed.message, false)
    case 'result': {
      const isError = parsed.is_error === true
      const text = str(parsed.result)
      return [
        {
          key: `${event.seq}.0`,
          kind: isError ? 'error' : 'result',
          title: isError ? 'Failed' : 'Finished',
          body: clip(text)
        }
      ]
    }
    default:
      return []
  }
}
