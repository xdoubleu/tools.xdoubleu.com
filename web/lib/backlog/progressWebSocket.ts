import { useEffect, useRef, useState } from 'react'

interface ProgressStatus {
  status: number
  lastMessage: string | null
}

export function useProgressWebSocket(url: string): ProgressStatus {
  const wsRef = useRef<WebSocket | null>(null)
  const [status, setStatus] = useState<number>(WebSocket.CONNECTING)
  const [lastMessage, setLastMessage] = useState<string | null>(null)

  useEffect(() => {
    if (!url) {
      return
    }

    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      setStatus(WebSocket.OPEN)
    }

    ws.onmessage = (event) => {
      setLastMessage(event.data)
    }

    ws.onerror = () => {
      setStatus(WebSocket.CLOSED)
    }

    ws.onclose = () => {
      setStatus(WebSocket.CLOSED)
    }

    return () => {
      if (wsRef.current) {
        wsRef.current.close()
      }
    }
  }, [url])

  return {
    status: status,
    lastMessage: lastMessage
  }
}
