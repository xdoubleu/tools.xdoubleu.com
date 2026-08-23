// Thin logger used in place of raw console.* calls. Always logs locally, and
// also fire-and-forgets a batched, debounced copy to the api's centralized
// log store (global.log_entries, issue #1040) so log history outlives a
// container restart instead of living only in stdout/Sentry.
//
// Runs both server- and client-side. Server-side, it can call the api's
// ingest endpoint directly (OBSERVABILITY_INGEST_SECRET is readable from
// process.env there). Client-side it must never see that secret, so it posts
// to this app's own /logs route instead, which attaches the header
// server-side before forwarding (app/logs/route.ts).
import { getApiUrl, getObservabilityIngestSecret } from './env'

type LogLevel = 'debug' | 'info' | 'warn' | 'error'

interface QueuedEntry {
  occurred_at: string
  level: string
  message: string
  attrs?: Record<string, unknown>
}

const FLUSH_DELAY_MS = 2000
const MAX_BATCH_SIZE = 25
const INGEST_PATH = '/api/observability/logs'

let queue: QueuedEntry[] = []
let flushTimer: ReturnType<typeof setTimeout> | null = null

function scheduleFlush() {
  if (flushTimer) return
  flushTimer = setTimeout(flush, FLUSH_DELAY_MS)
}

function flush() {
  flushTimer = null
  if (queue.length === 0) return
  const entries = queue
  queue = []
  void send(entries)
}

async function send(entries: QueuedEntry[]): Promise<void> {
  try {
    if (typeof window === 'undefined') {
      const secret = getObservabilityIngestSecret()
      if (!secret) return
      await fetch(`${getApiUrl()}${INGEST_PATH}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Observability-Ingest-Secret': secret
        },
        body: JSON.stringify({ entries })
      })
    } else {
      await fetch('/logs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ entries })
      })
    }
  } catch {
    // Best-effort: the local console output above already happened, and a
    // dropped log line isn't worth surfacing further.
  }
}

function enqueue(level: LogLevel, message: string, attrs?: Record<string, unknown>) {
  queue.push({ occurred_at: new Date().toISOString(), level, message, attrs })
  if (queue.length >= MAX_BATCH_SIZE) {
    if (flushTimer) {
      clearTimeout(flushTimer)
      flushTimer = null
    }
    flush()
  } else {
    scheduleFlush()
  }
}

function consoleFor(level: LogLevel): (...args: unknown[]) => void {
  switch (level) {
    case 'debug':
      return console.debug
    case 'warn':
      return console.warn
    case 'error':
      return console.error
    default:
      return console.info
  }
}

function log(level: LogLevel, message: string, attrs?: Record<string, unknown>) {
  const fn = consoleFor(level)
  if (attrs) fn(message, attrs)
  else fn(message)
  enqueue(level, message, attrs)
}

export const logger = {
  debug: (message: string, attrs?: Record<string, unknown>) => log('debug', message, attrs),
  info: (message: string, attrs?: Record<string, unknown>) => log('info', message, attrs),
  warn: (message: string, attrs?: Record<string, unknown>) => log('warn', message, attrs),
  error: (message: string, attrs?: Record<string, unknown>) => log('error', message, attrs)
}
