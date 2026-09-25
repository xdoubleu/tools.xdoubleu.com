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

// Pushes are the api's JSON DTO, shaped like the generated JourneyDetail.
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
 * useJourneyLive returns the latest pushed state for one journey. A locked
 * phone can suspend the socket without a close event, so becoming visible
 * (visibilitychange, pageshow, online) forces a fresh socket and calls
 * refetchDetail. The socket also reconnects after close/error.
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
