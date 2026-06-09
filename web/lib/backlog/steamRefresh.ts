import { useCallback, useEffect, useRef, useState } from 'react'
import { getApiUrl } from '@/lib/env'
import { useRefreshSteam } from '@/hooks/useBacklog'

const RECONNECT_DELAY_MS = 1500

interface StateMessage {
  isRefreshing: boolean
  lastRefresh: string | null
}

export interface SteamRefreshState {
  connected: boolean
  isRefreshing: boolean
  lastRefresh: Date | null
  refresh: () => void
}

function isStateMessage(value: unknown): value is StateMessage {
  return (
    value !== null &&
    typeof value === 'object' &&
    'isRefreshing' in value &&
    typeof value.isRefreshing === 'boolean'
  )
}

function buildProgressWsUrl(apiUrl: string): string {
  const wsBase = apiUrl
    .replace(/^https:\/\//, 'wss://')
    .replace(/^http:\/\//, 'ws://')
    .replace(/\/$/, '')
  return `${wsBase}/backlog/api/progress`
}

// useSteamRefresh subscribes to the backlog progress WebSocket for the "steam"
// topic, mirroring the original pre-migration refresh widget: it reports the
// live refreshing state and last refresh time, exposes a trigger that kicks off
// a server-side Steam sync, and invokes onSynced once a sync completes so the
// caller can re-fetch the freshly synced data.
export function useSteamRefresh(onSynced?: () => void): SteamRefreshState {
  const triggerRefresh = useRefreshSteam()
  const [connected, setConnected] = useState(false)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null)
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
      const ws = new WebSocket(buildProgressWsUrl(apiUrl))
      socket = ws

      ws.onopen = () => {
        setConnected(true)
        ws.send(JSON.stringify({ subject: 'steam' }))
      }

      ws.onmessage = (event: MessageEvent<string>) => {
        const parsed: unknown = JSON.parse(event.data)
        if (!isStateMessage(parsed)) return

        setIsRefreshing(parsed.isRefreshing)
        setLastRefresh(typeof parsed.lastRefresh === 'string' ? new Date(parsed.lastRefresh) : null)

        if (parsed.isRefreshing) {
          wasRefreshing.current = true
        } else if (wasRefreshing.current) {
          wasRefreshing.current = false
          onSyncedRef.current?.()
        }
      }

      // A sync can outlive an idle socket; reconnecting and re-subscribing means
      // the reconnect's initial state message still delivers the completion
      // (isRefreshing -> false), which re-enables the button and refetches.
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
  }, [])

  const refresh = useCallback(() => {
    void triggerRefresh()
  }, [triggerRefresh])

  return { connected, isRefreshing, lastRefresh, refresh }
}
