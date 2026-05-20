'use client'

import { useEffect, useRef, useState } from 'react'
import Link from 'next/link'
import { buildWsUrl } from '@/lib/watchparty/roomUtils'
import { getApiUrl } from '@/lib/env'

type ConnectionStatus = 'connecting' | 'connected' | 'disconnected'
type TrackType = 'cam' | 'screen'

interface WsMessage {
  type: 'offer' | 'answer' | 'candidate'
  payload: RTCSessionDescriptionInit | RTCIceCandidateInit
  trackType: TrackType
}

function attachDraggable(el: HTMLElement) {
  let startX = 0,
    startY = 0,
    origLeft = 0,
    origTop = 0

  const onDown = (e: PointerEvent) => {
    e.preventDefault()
    el.setPointerCapture(e.pointerId)
    el.style.cursor = 'grabbing'
    const container = el.parentElement!.getBoundingClientRect()
    const rect = el.getBoundingClientRect()
    el.style.right = 'auto'
    el.style.bottom = 'auto'
    el.style.left = `${rect.left - container.left}px`
    el.style.top = `${rect.top - container.top}px`
    startX = e.clientX
    startY = e.clientY
    origLeft = parseFloat(el.style.left)
    origTop = parseFloat(el.style.top)
  }
  const onMove = (e: PointerEvent) => {
    if (!el.hasPointerCapture(e.pointerId)) return
    el.style.left = `${origLeft + e.clientX - startX}px`
    el.style.top = `${origTop + e.clientY - startY}px`
  }
  const onUp = (e: PointerEvent) => {
    el.releasePointerCapture(e.pointerId)
    el.style.cursor = 'grab'
  }

  el.addEventListener('pointerdown', onDown)
  el.addEventListener('pointermove', onMove)
  el.addEventListener('pointerup', onUp)
  return () => {
    el.removeEventListener('pointerdown', onDown)
    el.removeEventListener('pointermove', onMove)
    el.removeEventListener('pointerup', onUp)
  }
}

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

export default function ViewerClient({ id }: { id: string }) {
  const mainVideoRef = useRef<HTMLVideoElement>(null)
  const selfCamRef = useRef<HTMLVideoElement>(null)
  const remoteCamRef = useRef<HTMLVideoElement>(null)

  const wsRef = useRef<WebSocket | null>(null)
  // pcCamRef: OUTGOING cam (viewer's cam sent to presenter)
  const pcCamRef = useRef<RTCPeerConnection | null>(null)
  // pcInCamRef: INCOMING cam (presenter's cam received by viewer)
  const pcInCamRef = useRef<RTCPeerConnection | null>(null)
  const pcScreenRef = useRef<RTCPeerConnection | null>(null)
  const localCamRef = useRef<MediaStream | null>(null)
  const remoteCamStreamRef = useRef<MediaStream | null>(null)
  const isSharingScreenRef = useRef(false)
  const pendingCandidates = useRef<{
    camOut: RTCIceCandidateInit[]
    camIn: RTCIceCandidateInit[]
    screen: RTCIceCandidateInit[]
  }>({
    camOut: [],
    camIn: [],
    screen: []
  })
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const [status, setStatus] = useState<ConnectionStatus>('connecting')
  const [micEnabled, setMicEnabled] = useState(true)
  const [camEnabled, setCamEnabled] = useState(true)
  const [selfCamVisible, setSelfCamVisible] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!selfCamRef.current) return
    const cleanDrag = attachDraggable(selfCamRef.current)
    return () => {
      cleanDrag()
    }
  }, [])

  useEffect(() => {
    if (!remoteCamRef.current) return
    const cleanDrag = attachDraggable(remoteCamRef.current)
    return () => {
      cleanDrag()
    }
  }, [])

  useEffect(() => {
    const apiUrl = getApiUrl()

    function send(type: string, payload: unknown, trackType: TrackType) {
      const msg = JSON.stringify({ type, payload, trackType })
      const ws = wsRef.current
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(msg)
      } else {
        setTimeout(() => send(type, payload, trackType), 200)
      }
    }

    function createPC(
      trackType: TrackType,
      direction: 'send' | 'recv' = 'recv'
    ): RTCPeerConnection {
      const pc = new RTCPeerConnection({
        iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
      })

      pc.onicecandidate = (e) => {
        if (e.candidate) send('candidate', e.candidate, trackType)
      }

      if (direction === 'recv') {
        pc.ontrack = (e) => {
          const mainEl = mainVideoRef.current
          const remoteEl = remoteCamRef.current
          if (trackType === 'cam') {
            remoteCamStreamRef.current = e.streams[0]
            if (isSharingScreenRef.current) {
              if (remoteEl) {
                remoteEl.srcObject = remoteCamStreamRef.current
                remoteEl.style.display = 'block'
              }
            } else {
              if (mainEl) mainEl.srcObject = remoteCamStreamRef.current
              if (remoteEl) remoteEl.style.display = 'none'
            }
          }
          if (trackType === 'screen') {
            isSharingScreenRef.current = true
            if (mainEl) mainEl.srcObject = e.streams[0]
            if (remoteEl) {
              remoteEl.srcObject = remoteCamStreamRef.current
              remoteEl.style.display = remoteCamStreamRef.current ? 'block' : 'none'
            }
          }
        }
      }

      pc.onconnectionstatechange = () => {
        if (trackType === 'cam' && direction === 'recv') {
          // pcInCamRef connection failed
          if (pc.connectionState === 'failed') {
            pcInCamRef.current = null
            pendingCandidates.current.camIn = []
          }
        }
        if (trackType === 'cam' && direction === 'send') {
          // pcCamRef connection failed — attempt to re-offer
          if (pc.connectionState === 'failed') {
            pcCamRef.current = null
            pendingCandidates.current.camOut = []
            // Trigger re-negotiation
            void (async () => {
              const localCam = localCamRef.current
              if (localCam) {
                const newPc = createPC('cam', 'send')
                pcCamRef.current = newPc
                localCam.getTracks().forEach((t) => newPc.addTrack(t, localCam))
                const offer = await newPc.createOffer()
                await newPc.setLocalDescription(offer)
                send('offer', offer, 'cam')
              }
            })()
          }
        }
        if (
          trackType === 'screen' &&
          (pc.connectionState === 'disconnected' || pc.connectionState === 'failed')
        ) {
          isSharingScreenRef.current = false
          pcScreenRef.current = null
          pendingCandidates.current.screen = []
          const mainEl = mainVideoRef.current
          const remoteEl = remoteCamRef.current
          if (remoteCamStreamRef.current && mainEl) mainEl.srcObject = remoteCamStreamRef.current
          if (remoteEl) remoteEl.style.display = 'none'
        }
      }

      return pc
    }

    async function renegotiateAll() {
      const oldCam = pcCamRef.current
      if (oldCam) {
        oldCam.close()
        pcCamRef.current = null
        pendingCandidates.current.camOut = []
      }
      const oldInCam = pcInCamRef.current
      if (oldInCam) {
        oldInCam.close()
        pcInCamRef.current = null
        pendingCandidates.current.camIn = []
      }
      const localCam = localCamRef.current
      if (localCam) {
        const pc = createPC('cam', 'send')
        pcCamRef.current = pc
        localCam.getTracks().forEach((t) => pc.addTrack(t, localCam))
        const offer = await pc.createOffer()
        await pc.setLocalDescription(offer)
        send('offer', offer, 'cam')
      }

      const oldScreen = pcScreenRef.current
      if (oldScreen) {
        oldScreen.close()
        pcScreenRef.current = null
        pendingCandidates.current.screen = []
      }
    }

    function initWS() {
      const wsUrl = buildWsUrl(apiUrl, id, false)
      const ws = new WebSocket(wsUrl)
      wsRef.current = ws

      ws.onopen = () => {
        setStatus('connected')
        ws.send(JSON.stringify({ roomCode: id, role: 'viewer' }))
        void renegotiateAll()
      }
      ws.onclose = () => {
        setStatus('disconnected')
        reconnectTimer.current = setTimeout(initWS, 1500)
      }
      ws.onerror = () => ws.close()

      ws.onmessage = async (event: MessageEvent<string>) => {
        const msg = JSON.parse(event.data) as WsMessage
        const tt = msg.trackType

        if (msg.type === 'offer') {
          if (tt === 'cam') {
            // Incoming cam offer from presenter → use pcInCamRef
            let pc = pcInCamRef.current
            if (
              pc &&
              (pc.signalingState === 'closed' ||
                pc.connectionState === 'failed' ||
                pc.connectionState === 'closed' ||
                pc.connectionState === 'disconnected')
            ) {
              pc.close()
              pc = null
              pcInCamRef.current = null
              pendingCandidates.current.camIn = []
            }
            if (!pc) {
              pc = createPC('cam', 'recv')
              pcInCamRef.current = pc
              pendingCandidates.current.camIn = []
            }
            await pc.setRemoteDescription(msg.payload as RTCSessionDescriptionInit)
            for (const c of pendingCandidates.current.camIn.splice(0)) await pc.addIceCandidate(c)
            const answer = await pc.createAnswer()
            await pc.setLocalDescription(answer)
            send('answer', answer, tt)
          } else {
            // Screen offer
            let pc = pcScreenRef.current
            if (
              pc &&
              (pc.signalingState === 'closed' ||
                pc.connectionState === 'failed' ||
                pc.connectionState === 'closed' ||
                pc.connectionState === 'disconnected')
            ) {
              pc.close()
              pc = null
              pcScreenRef.current = null
              pendingCandidates.current.screen = []
            }
            if (!pc) {
              pc = createPC(tt, 'recv')
              pcScreenRef.current = pc
              pendingCandidates.current[tt] = []
            }
            await pc.setRemoteDescription(msg.payload as RTCSessionDescriptionInit)
            for (const c of pendingCandidates.current[tt].splice(0)) await pc.addIceCandidate(c)
            const answer = await pc.createAnswer()
            await pc.setLocalDescription(answer)
            send('answer', answer, tt)
          }
        }

        if (msg.type === 'answer') {
          // Answer to our outgoing cam offer
          if (tt === 'cam') {
            const pc = pcCamRef.current
            if (pc && pc.signalingState === 'have-local-offer') {
              await pc.setRemoteDescription(msg.payload as RTCSessionDescriptionInit)
              for (const c of pendingCandidates.current.camOut.splice(0))
                await pc.addIceCandidate(c)
            }
          } else {
            const pc = pcScreenRef.current
            if (pc && pc.signalingState === 'have-local-offer') {
              await pc.setRemoteDescription(msg.payload as RTCSessionDescriptionInit)
              for (const c of pendingCandidates.current[tt].splice(0)) await pc.addIceCandidate(c)
            }
          }
        }

        if (msg.type === 'candidate') {
          if (tt === 'cam') {
            // Route to pcInCamRef if it has a remote description, else pcCamRef
            const inCam = pcInCamRef.current
            const outCam = pcCamRef.current
            if (inCam && inCam.remoteDescription) {
              await inCam.addIceCandidate(msg.payload as RTCIceCandidateInit)
            } else if (outCam && outCam.remoteDescription) {
              await outCam.addIceCandidate(msg.payload as RTCIceCandidateInit)
            } else if (inCam) {
              pendingCandidates.current.camIn.push(msg.payload as RTCIceCandidateInit)
            } else {
              pendingCandidates.current.camOut.push(msg.payload as RTCIceCandidateInit)
            }
          } else {
            const pc = pcScreenRef.current
            if (pc && pc.remoteDescription) {
              await pc.addIceCandidate(msg.payload as RTCIceCandidateInit)
            } else if (pc) {
              pendingCandidates.current[tt].push(msg.payload as RTCIceCandidateInit)
            }
          }
        }
      }
    }

    async function startCamera() {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true })
        localCamRef.current = stream
        if (selfCamRef.current) selfCamRef.current.srcObject = stream
        const pc = createPC('cam', 'send')
        pcCamRef.current = pc
        stream.getTracks().forEach((t) => pc.addTrack(t, stream))
        const offer = await pc.createOffer()
        await pc.setLocalDescription(offer)
        send('offer', offer, 'cam')
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to start camera')
      }
    }

    initWS()
    void startCamera()

    return () => {
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      wsRef.current?.close()
      pcCamRef.current?.close()
      pcInCamRef.current?.close()
      pcScreenRef.current?.close()
      localCamRef.current?.getTracks().forEach((t) => t.stop())
    }
  }, [id])

  function toggleMic() {
    const next = !micEnabled
    setMicEnabled(next)
    localCamRef.current?.getAudioTracks().forEach((t) => {
      t.enabled = next
    })
  }

  function toggleCam() {
    const next = !camEnabled
    setCamEnabled(next)
    localCamRef.current?.getVideoTracks().forEach((t) => {
      t.enabled = next
    })
    if (selfCamRef.current) {
      selfCamRef.current.style.display = next && selfCamVisible ? 'block' : 'none'
    }
  }

  function toggleSelfCam() {
    const next = !selfCamVisible
    setSelfCamVisible(next)
    if (selfCamRef.current) {
      selfCamRef.current.style.display = next && camEnabled ? 'block' : 'none'
    }
  }

  return (
    <div className="flex flex-col h-[calc(100vh-4rem)]">
      <div className="flex items-center gap-4 px-4 py-3 border-b border-border">
        <Link href="/watchparty" className="text-blue-600 hover:underline text-sm">
          &larr; Back
        </Link>
        <h1 className="text-xl font-bold">Watch Party</h1>
        <div className="flex items-center gap-2 ml-auto">
          <span className={`w-2.5 h-2.5 rounded-full ${STATUS_COLOR[status]}`} />
          <span className="text-sm text-muted-foreground">{STATUS_LABEL[status]}</span>
        </div>
      </div>

      {error && <p className="px-4 py-2 text-red-600 text-sm bg-red-50">{error}</p>}

      <div className="relative flex-1 bg-black overflow-hidden">
        <video ref={mainVideoRef} autoPlay playsInline className="w-full h-full object-contain" />

        <video
          ref={selfCamRef}
          autoPlay
          playsInline
          muted
          className="absolute bottom-4 right-4 rounded-lg object-cover border-2 border-white/40 cursor-grab bg-gray-900"
          style={{
            width: 200,
            height: 200,
            display: 'block',
            transform: 'scaleX(-1)'
          }}
        />

        <video
          ref={remoteCamRef}
          autoPlay
          playsInline
          className="absolute bottom-4 left-4 rounded-lg object-cover border-2 border-white/40 cursor-grab bg-gray-900"
          style={{
            width: 200,
            height: 200,
            display: 'none'
          }}
        />
      </div>

      <div className="flex flex-wrap items-center gap-2 px-4 py-3 border-t border-border bg-background">
        <button
          onClick={toggleMic}
          className={`px-4 py-1.5 rounded text-sm font-medium border ${
            micEnabled
              ? 'border-border hover:bg-accent'
              : 'bg-yellow-500 text-white border-yellow-500'
          }`}
        >
          {micEnabled ? 'Mute Mic' : 'Unmute Mic'}
        </button>

        <button
          onClick={toggleCam}
          className={`px-4 py-1.5 rounded text-sm font-medium border ${
            camEnabled
              ? 'border-border hover:bg-accent'
              : 'bg-yellow-500 text-white border-yellow-500'
          }`}
        >
          {camEnabled ? 'Disable Cam' : 'Enable Cam'}
        </button>

        <button
          onClick={toggleSelfCam}
          className={`px-4 py-1.5 rounded text-sm font-medium border ${
            selfCamVisible ? 'border-border hover:bg-accent' : 'border-border text-muted-foreground'
          }`}
        >
          {selfCamVisible ? 'Hide Self' : 'Show Self'}
        </button>

        <span className="ml-auto text-xs text-muted-foreground font-mono">Room: {id}</span>
      </div>
    </div>
  )
}
