import { useEffect, useReducer, useRef, useState } from 'react'
import { getApiUrl } from '@/lib/env'
import {
  applyFrame,
  chatSocketUrl,
  EMPTY_TRANSCRIPT,
  isChatFrame,
  type ChatTranscript
} from '@/lib/factory/chat'

const RECONNECT_DELAY_MS = 1500

export interface ChatSocket {
  transcript: ChatTranscript
  /** The session's state; empty until the first state frame. */
  state: string
  connected: boolean
}

/**
 * A chat's transcript over its socket. A dropped socket (reload, phone sleep)
 * redials with the cursor of the last seq seen, so the server replays only what
 * was missed. It stops redialling once the chat is closed.
 */
export function useChatSocket(id: string | null): ChatSocket {
  const [transcript, push] = useReducer(applyFrame, EMPTY_TRANSCRIPT)
  const [state, setState] = useState('')
  const [connected, setConnected] = useState(false)
  const cursor = useRef(0)
  cursor.current = transcript.cursor

  useEffect(() => {
    const apiUrl = getApiUrl()
    if (!id || !apiUrl) return
    let socket: WebSocket | null = null
    let timer: ReturnType<typeof setTimeout> | null = null
    let stopped = false
    let closed = false

    const connect = () => {
      const ws = new WebSocket(chatSocketUrl(apiUrl, id, cursor.current))
      socket = ws
      ws.onopen = () => setConnected(true)
      ws.onmessage = (event: MessageEvent<string>) => {
        let parsed: unknown
        try {
          parsed = JSON.parse(event.data)
        } catch {
          return
        }
        if (!isChatFrame(parsed)) return
        if (parsed.kind === 'state') {
          setState(parsed.state ?? '')
          if (parsed.state === 'stopped') closed = true
          return
        }
        cursor.current = Math.max(cursor.current, parsed.seq)
        push(parsed)
      }
      ws.onclose = () => {
        setConnected(false)
        if (!stopped && !closed) timer = setTimeout(connect, RECONNECT_DELAY_MS)
      }
      ws.onerror = () => ws.close()
    }

    connect()
    return () => {
      stopped = true
      if (timer) clearTimeout(timer)
      socket?.close()
    }
  }, [id])

  return { transcript, state, connected }
}
