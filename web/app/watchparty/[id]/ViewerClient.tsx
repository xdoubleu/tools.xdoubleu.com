'use client'

import { useEffect, useRef, useState } from 'react'
import Link from 'next/link'
import { buildWsUrl } from '@/lib/watchparty/roomUtils'

type ConnectionStatus = 'connecting' | 'connected' | 'disconnected'

const STATUS_LABEL: Record<ConnectionStatus, string> = {
  connecting: 'Connecting...',
  connected: 'Connected',
  disconnected: 'Disconnected',
}

const STATUS_COLOR: Record<ConnectionStatus, string> = {
  connecting: 'bg-yellow-400',
  connected: 'bg-green-500',
  disconnected: 'bg-red-500',
}

export default function ViewerClient({ id }: { id: string }) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const pcRef = useRef<RTCPeerConnection | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const [status, setStatus] = useState<ConnectionStatus>('connecting')

  useEffect(() => {
    const apiUrl = process.env.NEXT_PUBLIC_API_URL ?? ''
    const wsUrl = buildWsUrl(apiUrl, id, false)

    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    const pc = new RTCPeerConnection({
      iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
    })
    pcRef.current = pc

    pc.ontrack = (event) => {
      if (videoRef.current && event.streams[0]) {
        videoRef.current.srcObject = event.streams[0]
      }
    }

    pc.onicecandidate = (event) => {
      if (event.candidate && ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({
            type: 'candidate',
            payload: event.candidate,
            trackType: '',
          })
        )
      }
    }

    ws.onopen = () => {
      setStatus('connected')
      ws.send(JSON.stringify({ roomCode: id, role: 'viewer' }))
    }

    ws.onmessage = async (event: MessageEvent<string>) => {
      const msg = JSON.parse(event.data) as {
        type: 'offer' | 'answer' | 'candidate'
        payload: RTCSessionDescriptionInit | RTCIceCandidateInit
        trackType: string
      }
      if (msg.type === 'offer') {
        await pc.setRemoteDescription(
          new RTCSessionDescription(msg.payload as RTCSessionDescriptionInit)
        )
        const answer = await pc.createAnswer()
        await pc.setLocalDescription(answer)
        ws.send(
          JSON.stringify({ type: 'answer', payload: answer, trackType: '' })
        )
      } else if (msg.type === 'candidate') {
        await pc.addIceCandidate(
          new RTCIceCandidate(msg.payload as RTCIceCandidateInit)
        )
      }
    }

    ws.onclose = () => setStatus('disconnected')
    ws.onerror = () => setStatus('disconnected')

    return () => {
      ws.close()
      pc.close()
    }
  }, [id])

  return (
    <main className="max-w-4xl mx-auto p-6">
      <div className="flex items-center gap-4 mb-4">
        <Link href="/watchparty" className="text-blue-600 hover:underline text-sm">
          &larr; Back
        </Link>
        <h1 className="text-2xl font-bold">Watch Party</h1>
        <div className="flex items-center gap-2 ml-auto">
          <span
            className={`w-2.5 h-2.5 rounded-full ${STATUS_COLOR[status]}`}
          />
          <span className="text-sm text-gray-600">{STATUS_LABEL[status]}</span>
        </div>
      </div>

      <div className="text-sm text-gray-500 mb-4">
        Room: <span className="font-mono font-medium">{id}</span>
      </div>

      <div className="bg-black rounded-lg overflow-hidden aspect-video">
        <video
          ref={videoRef}
          autoPlay
          playsInline
          className="w-full h-full object-contain"
        />
      </div>
    </main>
  )
}
