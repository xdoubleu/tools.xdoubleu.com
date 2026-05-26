'use client'

import { useEffect, useRef, useState } from 'react'
import { buildWsUrl } from '@/lib/watchparty/roomUtils'
import { getApiUrl } from '@/lib/env'
import type { ConnectionStatus, TrackType, WsMessage } from '@/lib/watchparty/types'

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

    function send(type: string, payload: unknown, trackType: TrackType, direction?: string) {
      const msg = JSON.stringify({ type, payload, trackType, direction })
      const ws = wsRef.current
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(msg)
      } else {
        setTimeout(() => send(type, payload, trackType, direction), 200)
      }
    }

    function stopScreenUI() {
      isSharingScreenRef.current = false
      onSharingChange?.(false)
      const mainEl = mainVideoRef.current
      const remoteEl = remoteCamRef.current
      if (remoteCamStreamRef.current && mainEl) mainEl.srcObject = remoteCamStreamRef.current
      if (remoteEl) remoteEl.style.display = 'none'
    }

    function createPC(
      trackType: TrackType,
      direction: 'send' | 'recv' = 'recv'
    ): RTCPeerConnection {
      const pc = new RTCPeerConnection({
        iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
      })

      pc.onicecandidate = (e) => {
        if (e.candidate) send('candidate', e.candidate, trackType, direction)
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
            if (role === 'presenter') onSharingChange?.(true)
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
          if (pc.connectionState === 'failed') {
            pcInCamRef.current = null
            pendingCandidates.current.camIn = []
          }
        }
        if (trackType === 'cam' && direction === 'send') {
          if (pc.connectionState === 'failed') {
            pcCamRef.current = null
            pendingCandidates.current.camOut = []
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
          if (role === 'presenter') {
            stopScreenUI()
          } else {
            isSharingScreenRef.current = false
            pcScreenRef.current = null
            pendingCandidates.current.screen = []
            const mainEl = mainVideoRef.current
            const remoteEl = remoteCamRef.current
            if (remoteCamStreamRef.current && mainEl) mainEl.srcObject = remoteCamStreamRef.current
            if (remoteEl) remoteEl.style.display = 'none'
          }
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
      const localCam = localCamRef.current
      if (localCam) {
        const pc = createPC('cam', 'send')
        pcCamRef.current = pc
        localCam.getTracks().forEach((t) => pc.addTrack(t, localCam))
        const offer = await pc.createOffer()
        await pc.setLocalDescription(offer)
        send('offer', offer, 'cam')
      }

      if (role === 'presenter') {
        const oldScreen = pcScreenRef.current
        if (oldScreen) {
          oldScreen.close()
          pcScreenRef.current = null
          pendingCandidates.current.screen = []
        }
        const localScreen = localScreenRef.current
        if (isSharingScreenRef.current && localScreen) {
          const pc = createPC('screen', 'send')
          pcScreenRef.current = pc
          localScreen.getTracks().forEach((t) => pc.addTrack(t, localScreen))
          const offer = await pc.createOffer()
          await pc.setLocalDescription(offer)
          send('offer', offer, 'screen')
        }
      }
    }

    // Presenter-only: screen sharing
    async function startScreen() {
      setError(null)
      try {
        const stream = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: true })
        localScreenRef.current = stream
        isSharingScreenRef.current = true
        onSharingChange?.(true)

        if (mainVideoRef.current) mainVideoRef.current.srcObject = stream
        const remoteEl = remoteCamRef.current
        if (remoteEl) {
          remoteEl.srcObject = remoteCamStreamRef.current
          remoteEl.style.display = remoteCamStreamRef.current ? 'block' : 'none'
        }

        const pc = createPC('screen', 'send')
        pcScreenRef.current = pc
        stream.getTracks().forEach((t) => pc.addTrack(t, stream))
        const offer = await pc.createOffer()
        await pc.setLocalDescription(offer)
        send('offer', offer, 'screen')

        stream.getVideoTracks()[0].onended = stopScreen
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to share screen')
      }
    }

    function stopScreen() {
      localScreenRef.current?.getTracks().forEach((t) => t.stop())
      localScreenRef.current = null
      if (pcScreenRef.current) {
        pcScreenRef.current.close()
        pcScreenRef.current = null
      }
      stopScreenUI()
    }

    if (screenControls) {
      screenControls.current = { start: startScreen, stop: stopScreen }
    }

    function initWS() {
      const wsUrl = buildWsUrl(apiUrl, id, role === 'presenter')
      const ws = new WebSocket(wsUrl)
      wsRef.current = ws

      ws.onopen = () => {
        setStatus('connected')
        ws.send(JSON.stringify({ roomCode: id, role }))
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
            if (pcInCamRef.current) pcInCamRef.current.close()
            pendingCandidates.current.camIn = []
            const pc = createPC('cam', 'recv')
            pcInCamRef.current = pc

            await pc.setRemoteDescription(msg.payload as RTCSessionDescriptionInit)
            if (pcInCamRef.current !== pc) return
            for (const c of pendingCandidates.current.camIn.splice(0)) await pc.addIceCandidate(c)
            if (pcInCamRef.current !== pc) return
            const answer = await pc.createAnswer()
            if (pcInCamRef.current !== pc) return
            await pc.setLocalDescription(answer)
            if (pcInCamRef.current !== pc) return
            send('answer', answer, tt)
          } else {
            if (pcScreenRef.current) pcScreenRef.current.close()
            pendingCandidates.current.screen = []
            const pcScreen = createPC(tt, 'recv')
            pcScreenRef.current = pcScreen

            await pcScreen.setRemoteDescription(msg.payload as RTCSessionDescriptionInit)
            if (pcScreenRef.current !== pcScreen) return
            for (const c of pendingCandidates.current[tt].splice(0))
              await pcScreen.addIceCandidate(c)
            if (pcScreenRef.current !== pcScreen) return
            const answer = await pcScreen.createAnswer()
            if (pcScreenRef.current !== pcScreen) return
            await pcScreen.setLocalDescription(answer)
            if (pcScreenRef.current !== pcScreen) return
            send('answer', answer, tt)
          }
        }

        if (msg.type === 'answer') {
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
            } else if (
              role === 'presenter' &&
              isSharingScreenRef.current &&
              localScreenRef.current
            ) {
              if (pcScreenRef.current) {
                pcScreenRef.current.close()
                pcScreenRef.current = null
              }
              pendingCandidates.current.screen = []
              const newPc = createPC('screen', 'send')
              pcScreenRef.current = newPc
              localScreenRef.current
                .getTracks()
                .forEach((t) => newPc.addTrack(t, localScreenRef.current!))
              const offer = await newPc.createOffer()
              await newPc.setLocalDescription(offer)
              send('offer', offer, 'screen')
            }
          }
        }

        if (msg.type === 'candidate') {
          if (tt === 'cam') {
            if (msg.direction === 'send') {
              const inCam = pcInCamRef.current
              if (inCam && inCam.remoteDescription) {
                await inCam.addIceCandidate(msg.payload as RTCIceCandidateInit)
              } else {
                pendingCandidates.current.camIn.push(msg.payload as RTCIceCandidateInit)
              }
            } else {
              const outCam = pcCamRef.current
              if (outCam && outCam.remoteDescription) {
                await outCam.addIceCandidate(msg.payload as RTCIceCandidateInit)
              } else {
                pendingCandidates.current.camOut.push(msg.payload as RTCIceCandidateInit)
              }
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
        if (wsRef.current?.readyState === WebSocket.OPEN) {
          void renegotiateAll()
        }
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
