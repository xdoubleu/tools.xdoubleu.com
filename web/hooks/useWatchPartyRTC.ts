'use client'

import { useEffect, useRef, useState } from 'react'
import { buildWsUrl } from '@/lib/watchparty/roomUtils'
import { getApiUrl } from '@/lib/env'
import { createMediaController } from '@/lib/watchparty/rtcMedia'
import type { RTCRefs } from '@/lib/watchparty/rtcMedia'
import { createSignalHandler } from '@/lib/watchparty/rtcSignaling'
import type { ConnectionStatus, TrackType } from '@/lib/watchparty/types'

export interface ScreenControls {
  start: () => Promise<void>
  stop: () => void
}

interface UseWatchPartyRTCOptions {
  id: string
  role: 'presenter' | 'viewer'
  mainVideoRef: React.RefObject<HTMLVideoElement | null>
  selfCamRef: React.RefObject<HTMLVideoElement | null>
  remoteCamRef: React.RefObject<HTMLVideoElement | null>
  onSharingChange?: (sharing: boolean) => void
  screenControls?: React.RefObject<ScreenControls>
}

interface UseWatchPartyRTCResult {
  status: ConnectionStatus
  micEnabled: boolean
  camEnabled: boolean
  selfCamVisible: boolean
  error: string | null
  toggleMic: () => void
  toggleCam: () => void
  toggleSelfCam: () => void
}

// useWatchPartyRTC wires the browser side of a watch-party room: media and
// peer-connection management lives in lib/watchparty/rtcMedia, WebSocket
// signalling in lib/watchparty/rtcSignaling; this hook owns the React state,
// the mutable session refs, and the socket lifecycle (connect + reconnect).
export function useWatchPartyRTC({
  id,
  role,
  mainVideoRef,
  selfCamRef,
  remoteCamRef,
  onSharingChange,
  screenControls
}: UseWatchPartyRTCOptions): UseWatchPartyRTCResult {
  const wsRef = useRef<WebSocket | null>(null)
  const pcCamRef = useRef<RTCPeerConnection | null>(null)
  const pcInCamRef = useRef<RTCPeerConnection | null>(null)
  const pcScreenRef = useRef<RTCPeerConnection | null>(null)
  const localCamRef = useRef<MediaStream | null>(null)
  const localScreenRef = useRef<MediaStream | null>(null)
  const remoteCamStreamRef = useRef<MediaStream | null>(null)
  const isSharingScreenRef = useRef(false)
  const pendingCandidates = useRef<{
    camOut: RTCIceCandidateInit[]
    camIn: RTCIceCandidateInit[]
    screen: RTCIceCandidateInit[]
  }>({ camOut: [], camIn: [], screen: [] })
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const [status, setStatus] = useState<ConnectionStatus>('connecting')
  const [micEnabled, setMicEnabled] = useState(true)
  const [camEnabled, setCamEnabled] = useState(true)
  const [selfCamVisible, setSelfCamVisible] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const apiUrl = getApiUrl()

    const refs: RTCRefs = {
      ws: wsRef,
      pcCam: pcCamRef,
      pcInCam: pcInCamRef,
      pcScreen: pcScreenRef,
      localCam: localCamRef,
      localScreen: localScreenRef,
      remoteCamStream: remoteCamStreamRef,
      isSharingScreen: isSharingScreenRef,
      pendingCandidates
    }

    function send(type: string, payload: unknown, trackType: TrackType, direction?: string) {
      const msg = JSON.stringify({ type, payload, trackType, direction })
      const ws = wsRef.current
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(msg)
      } else {
        setTimeout(() => send(type, payload, trackType, direction), 200)
      }
    }

    const media = createMediaController({
      refs,
      role,
      send,
      mainVideoRef,
      selfCamRef,
      remoteCamRef,
      onSharingChange,
      setError
    })

    const onSignal = createSignalHandler({ refs, role, send, createPC: media.createPC })

    if (screenControls) {
      screenControls.current = { start: media.startScreen, stop: media.stopScreen }
    }

    function initWS() {
      const wsUrl = buildWsUrl(apiUrl, id, role === 'presenter')
      const ws = new WebSocket(wsUrl)
      wsRef.current = ws

      ws.onopen = () => {
        setStatus('connected')
        ws.send(JSON.stringify({ roomCode: id, role }))
        void media.renegotiateAll()
      }
      ws.onclose = () => {
        setStatus('disconnected')
        reconnectTimer.current = setTimeout(initWS, 1500)
      }
      ws.onerror = () => ws.close()
      ws.onmessage = onSignal
    }

    initWS()
    void media.startCamera()

    return () => {
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      wsRef.current?.close()
      pcCamRef.current?.close()
      pcInCamRef.current?.close()
      pcScreenRef.current?.close()
      localCamRef.current?.getTracks().forEach((t) => t.stop())
      localScreenRef.current?.getTracks().forEach((t) => t.stop())
    }
  }, [id, role])

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

  return {
    status,
    micEnabled,
    camEnabled,
    selfCamVisible,
    error,
    toggleMic,
    toggleCam,
    toggleSelfCam
  }
}
