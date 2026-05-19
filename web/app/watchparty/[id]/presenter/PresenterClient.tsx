'use client'

import { useEffect, useRef, useState } from 'react'
import Link from 'next/link'
import { buildWsUrl } from '@/lib/watchparty/roomUtils'

type ConnectionStatus = 'connecting' | 'connected' | 'disconnected'

const STATUS_LABEL: Record<ConnectionStatus, string> = {
  connecting: 'Connecting...',
  connected: 'Connected',
  disconnected: 'Disconnected'
}

const STATUS_COLOR: Record<ConnectionStatus, string> = {
  connecting: 'bg-yellow-400',
  connected: 'bg-green-500',
  disconnected: 'bg-red-500'
}

export default function PresenterClient({ id }: { id: string }) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const pcRef = useRef<RTCPeerConnection | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const [status, setStatus] = useState<ConnectionStatus>('connecting')
  const [sharing, setSharing] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const apiUrl = process.env.NEXT_PUBLIC_API_URL ?? ''
    const wsUrl = buildWsUrl(apiUrl, id, true)

    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    const pc = new RTCPeerConnection({
      iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
    })
    pcRef.current = pc

    pc.onicecandidate = (event) => {
      if (event.candidate && ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({
            type: 'candidate',
            payload: event.candidate,
            trackType: ''
          })
        )
      }
    }

    ws.onopen = () => {
      setStatus('connected')
      ws.send(JSON.stringify({ roomCode: id, role: 'presenter' }))
    }

    ws.onmessage = async (event: MessageEvent<string>) => {
      const msg = JSON.parse(event.data) as {
        type: 'offer' | 'answer' | 'candidate'
        payload: RTCSessionDescriptionInit | RTCIceCandidateInit
        trackType: string
      }
      if (msg.type === 'answer') {
        await pc.setRemoteDescription(
          new RTCSessionDescription(msg.payload as RTCSessionDescriptionInit)
        )
      } else if (msg.type === 'candidate') {
        await pc.addIceCandidate(new RTCIceCandidate(msg.payload as RTCIceCandidateInit))
      }
    }

    ws.onclose = () => setStatus('disconnected')
    ws.onerror = () => setStatus('disconnected')

    return () => {
      streamRef.current?.getTracks().forEach((t) => t.stop())
      ws.close()
      pc.close()
    }
  }, [id])

  async function startSharing() {
    setError(null)
    try {
      const stream = await navigator.mediaDevices.getDisplayMedia({
        video: true,
        audio: true
      })
      streamRef.current = stream

      if (videoRef.current) {
        videoRef.current.srcObject = stream
      }

      const pc = pcRef.current
      const ws = wsRef.current
      if (!pc || !ws) return

      stream.getTracks().forEach((track) => {
        pc.addTrack(track, stream)
      })

      const offer = await pc.createOffer()
      await pc.setLocalDescription(offer)

      ws.send(JSON.stringify({ type: 'offer', payload: offer, trackType: 'screen' }))

      setSharing(true)

      stream.getVideoTracks()[0]?.addEventListener('ended', () => {
        setSharing(false)
        streamRef.current = null
        if (videoRef.current) videoRef.current.srcObject = null
      })
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to capture screen')
    }
  }

  function stopSharing() {
    streamRef.current?.getTracks().forEach((t) => t.stop())
    streamRef.current = null
    if (videoRef.current) videoRef.current.srcObject = null
    setSharing(false)
  }

  return (
    <main className="max-w-4xl mx-auto p-6">
      <div className="flex items-center gap-4 mb-4">
        <Link href="/watchparty" className="text-blue-600 hover:underline text-sm">
          &larr; Back
        </Link>
        <h1 className="text-2xl font-bold">Presenting</h1>
        <div className="flex items-center gap-2 ml-auto">
          <span className={`w-2.5 h-2.5 rounded-full ${STATUS_COLOR[status]}`} />
          <span className="text-sm text-gray-600">{STATUS_LABEL[status]}</span>
        </div>
      </div>

      <div className="text-sm text-gray-500 mb-4">
        Room: <span className="font-mono font-medium">{id}</span>
      </div>

      <div className="bg-black rounded-lg overflow-hidden aspect-video mb-4">
        <video ref={videoRef} autoPlay playsInline muted className="w-full h-full object-contain" />
      </div>

      {!sharing ? (
        <button
          onClick={startSharing}
          disabled={status !== 'connected'}
          className="px-6 py-2 bg-blue-600 text-white rounded font-semibold hover:bg-blue-700 disabled:opacity-50"
        >
          Share Screen
        </button>
      ) : (
        <button
          onClick={stopSharing}
          className="px-6 py-2 bg-red-600 text-white rounded font-semibold hover:bg-red-700"
        >
          Stop Sharing
        </button>
      )}

      {error && <p className="mt-3 text-red-600 text-sm">{error}</p>}
    </main>
  )
}
