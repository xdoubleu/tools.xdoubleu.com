import { useEffect, useRef, useState } from 'react'
import { getApiUrl } from '@/lib/env'
import type { JourneyDetail } from '@/lib/gen/trains/v1/trains_pb'

const RECONNECT_DELAY_MS = 1500

function buildJourneyWsUrl(apiUrl: string): string {
  const wsBase = apiUrl
    .replace(/^https:\/\//, 'wss://')
    .replace(/^http:\/\//, 'ws://')
    .replace(/\/$/, '')
  return `${wsBase}/trains/api/journeys/live`
}

// The websocket push is the trains api's own JSON DTO (models.JourneyDetail's
// MarshalJSON, following internal/progressws' convention of a plain wire DTO
// rather than protobuf-encoding a websocket payload) — deliberately built to
// the same camelCase field shape as the generated JourneyDetail type, so a
// pushed message can be treated as one structurally.
function isJourneyDetail(value: unknown): value is JourneyDetail {
  return (
    value !== null &&
    typeof value === 'object' &&
    'legs' in value &&
    Array.isArray((value as { legs: unknown }).legs)
  )
}

export interface JourneyLiveState {
  connected: boolean
  pushedDetail: JourneyDetail | null
}

/**
 * useJourneyLive subscribes to one journey's live-update websocket topic
 * (issue #1394) and hands back whatever the server most recently pushed —
 * either the just-subscribed snapshot or a later realtime-poll-driven
 * update.
 *
 * Two things a naive socket gets wrong are handled explicitly here:
 *  - a phone locking its screen suspends the socket without necessarily
 *    firing a close event; on the page becoming visible/foregrounded again
 *    (visibilitychange, pageshow, online) this forces a fresh socket AND
 *    calls refetchDetail, so the page shows the current state immediately
 *    rather than silently keeping a stale one until the next push (or
 *    forever, if the suspended socket never actually closes).
 *  - the socket itself always reconnects on close/error after a fixed delay.
 */
export function useJourneyLive(
  journeyId: string,
  refetchDetail: () => Promise<unknown>
): JourneyLiveState {
  const [connected, setConnected] = useState(false)
  const [pushedDetail, setPushedDetail] = useState<JourneyDetail | null>(null)
  const refetchRef = useRef(refetchDetail)
  refetchRef.current = refetchDetail

  useEffect(() => {
    const apiUrl = getApiUrl()
    if (!apiUrl || !journeyId) return

    let socket: WebSocket | null = null
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null
    let stopped = false

    const connect = () => {
      const ws = new WebSocket(buildJourneyWsUrl(apiUrl))
      socket = ws

      ws.onopen = () => {
        setConnected(true)
        ws.send(JSON.stringify({ subject: journeyId }))
      }
      ws.onmessage = (event: MessageEvent<string>) => {
        let parsed: unknown
        try {
          parsed = JSON.parse(event.data)
        } catch {
          return
        }
        if (isJourneyDetail(parsed)) setPushedDetail(parsed)
      }
      ws.onclose = () => {
        setConnected(false)
        if (!stopped) reconnectTimer = setTimeout(connect, RECONNECT_DELAY_MS)
      }
      ws.onerror = () => ws.close()
    }

    const reconnectAndRefetch = () => {
      if (reconnectTimer) clearTimeout(reconnectTimer)
      socket?.close()
      connect()
      void refetchRef.current()
    }

    const onVisibilityChange = () => {
      if (document.visibilityState === 'visible') reconnectAndRefetch()
    }

    connect()
    document.addEventListener('visibilitychange', onVisibilityChange)
    window.addEventListener('pageshow', reconnectAndRefetch)
    window.addEventListener('online', reconnectAndRefetch)

    return () => {
      stopped = true
      if (reconnectTimer) clearTimeout(reconnectTimer)
      document.removeEventListener('visibilitychange', onVisibilityChange)
      window.removeEventListener('pageshow', reconnectAndRefetch)
      window.removeEventListener('online', reconnectAndRefetch)
      socket?.close()
    }
  }, [journeyId])

  return { connected, pushedDetail }
}
