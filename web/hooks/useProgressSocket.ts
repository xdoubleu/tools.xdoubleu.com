import { useCallback, useEffect, useRef, useState } from 'react'
import { getApiUrl } from '@/lib/env'

const RECONNECT_DELAY_MS = 1500

interface StateMessage {
  isRefreshing: boolean
  lastRefresh: string | null
  processed?: number
  total?: number
}

export interface ProgressState {
  connected: boolean
  isRefreshing: boolean
  lastRefresh: Date | null
  processed: number | null
  total: number | null
  refresh: () => void
}

function isStateMessage(value: unknown): value is StateMessage {
  return (
    value !== null &&
    typeof value === 'object' &&
    'isRefreshing' in value &&
    typeof (value as Record<string, unknown>).isRefreshing === 'boolean'
  )
}

export type ProgressApp = 'games' | 'books'

function buildProgressWsUrl(apiUrl: string, app: ProgressApp): string {
  const wsBase = apiUrl
    .replace(/^https:\/\//, 'wss://')
    .replace(/^http:\/\//, 'ws://')
    .replace(/\/$/, '')
  return `${wsBase}/${app}/api/progress`
}

// useProgressSocket subscribes to one job-progress topic and calls onSynced
// when a run completes.
export function useProgressSocket(
  app: ProgressApp,
  topic: string,
  triggerRefresh: () => Promise<unknown>,
  onSynced?: () => void
): ProgressState {
  const [connected, setConnected] = useState(false)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null)
  const [processed, setProcessed] = useState<number | null>(null)
  const [total, setTotal] = useState<number | null>(null)

  const wasRefreshing = useRef(false)
  const onSyncedRef = useRef(onSynced)
  onSyncedRef.current = onSynced

  useEffect(() => {
    const apiUrl = getApiUrl()
    if (!apiUrl) return

    let socket: WebSocket | null = null
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null
    let stopped = false

    const connect = () => {
      const ws = new WebSocket(buildProgressWsUrl(apiUrl, app))
      socket = ws

      ws.onopen = () => {
        setConnected(true)
        ws.send(JSON.stringify({ subject: topic }))
      }

      ws.onmessage = (event: MessageEvent<string>) => {
        const parsed: unknown = JSON.parse(event.data)
        if (!isStateMessage(parsed)) return

        setIsRefreshing(parsed.isRefreshing)
        setLastRefresh(typeof parsed.lastRefresh === 'string' ? new Date(parsed.lastRefresh) : null)

        if (parsed.isRefreshing) {
          wasRefreshing.current = true
          setProcessed(parsed.processed ?? null)
          setTotal(parsed.total ?? null)
        } else {
          setProcessed(null)
          setTotal(null)
          if (wasRefreshing.current) {
            wasRefreshing.current = false
            onSyncedRef.current?.()
          }
        }
      }

      // A sync can outlive the socket; the reconnect's initial state still
      // delivers completion.
      ws.onclose = () => {
        setConnected(false)
        if (!stopped) {
          reconnectTimer = setTimeout(connect, RECONNECT_DELAY_MS)
        }
      }
      ws.onerror = () => ws.close()
    }

    connect()

    return () => {
      stopped = true
      if (reconnectTimer) clearTimeout(reconnectTimer)
      socket?.close()
    }
  }, [app, topic])

  const refresh = useCallback(() => {
    void triggerRefresh()
  }, [triggerRefresh])

  return { connected, isRefreshing, lastRefresh, processed, total, refresh }
}
