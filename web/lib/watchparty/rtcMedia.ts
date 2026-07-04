import type { TrackType } from '@/lib/watchparty/types'

// Plain `{ current }` boxes so these modules stay React-free; the hook passes
// its useRef objects straight in.
export interface RTCRefs {
  ws: { current: WebSocket | null }
  /** Outgoing cam connection (our cam → peer). */
  pcCam: { current: RTCPeerConnection | null }
  /** Incoming cam connection (peer's cam → us). */
  pcInCam: { current: RTCPeerConnection | null }
  pcScreen: { current: RTCPeerConnection | null }
  localCam: { current: MediaStream | null }
  localScreen: { current: MediaStream | null }
  remoteCamStream: { current: MediaStream | null }
  isSharingScreen: { current: boolean }
  pendingCandidates: {
    current: {
      camOut: RTCIceCandidateInit[]
      camIn: RTCIceCandidateInit[]
      screen: RTCIceCandidateInit[]
    }
  }
}

export type SendFn = (
  type: string,
  payload: unknown,
  trackType: TrackType,
  direction?: string
) => void

export interface MediaControllerDeps {
  refs: RTCRefs
  role: 'presenter' | 'viewer'
  send: SendFn
  mainVideoRef: { current: HTMLVideoElement | null }
  selfCamRef: { current: HTMLVideoElement | null }
  remoteCamRef: { current: HTMLVideoElement | null }
  onSharingChange?: (sharing: boolean) => void
  setError: (error: string | null) => void
}

export interface MediaController {
  createPC: (trackType: TrackType, direction?: 'send' | 'recv') => RTCPeerConnection
  renegotiateAll: () => Promise<void>
  startScreen: () => Promise<void>
  stopScreen: () => void
  startCamera: () => Promise<void>
}

// createMediaController owns peer-connection construction (including the
// ontrack UI wiring and failure recovery) and local media capture. Signalling
// messages are handled separately by createSignalHandler.
export function createMediaController(deps: MediaControllerDeps): MediaController {
  const { refs, role, send, mainVideoRef, selfCamRef, remoteCamRef, onSharingChange, setError } =
    deps

  function stopScreenUI() {
    refs.isSharingScreen.current = false
    onSharingChange?.(false)
    const mainEl = mainVideoRef.current
    const remoteEl = remoteCamRef.current
    if (refs.remoteCamStream.current && mainEl) mainEl.srcObject = refs.remoteCamStream.current
    if (remoteEl) remoteEl.style.display = 'none'
  }

  function createPC(trackType: TrackType, direction: 'send' | 'recv' = 'recv'): RTCPeerConnection {
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
          refs.remoteCamStream.current = e.streams[0]
          if (refs.isSharingScreen.current) {
            if (remoteEl) {
              remoteEl.srcObject = refs.remoteCamStream.current
              remoteEl.style.display = 'block'
            }
          } else {
            if (mainEl) mainEl.srcObject = refs.remoteCamStream.current
            if (remoteEl) remoteEl.style.display = 'none'
          }
        }
        if (trackType === 'screen') {
          refs.isSharingScreen.current = true
          if (role === 'presenter') onSharingChange?.(true)
          if (mainEl) mainEl.srcObject = e.streams[0]
          if (remoteEl) {
            remoteEl.srcObject = refs.remoteCamStream.current
            remoteEl.style.display = refs.remoteCamStream.current ? 'block' : 'none'
          }
        }
      }
    }

    pc.onconnectionstatechange = () => {
      if (trackType === 'cam' && direction === 'recv') {
        if (pc.connectionState === 'failed') {
          refs.pcInCam.current = null
          refs.pendingCandidates.current.camIn = []
        }
      }
      if (trackType === 'cam' && direction === 'send') {
        if (pc.connectionState === 'failed') {
          refs.pcCam.current = null
          refs.pendingCandidates.current.camOut = []
          void (async () => {
            const localCam = refs.localCam.current
            if (localCam) {
              const newPc = createPC('cam', 'send')
              refs.pcCam.current = newPc
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
          refs.isSharingScreen.current = false
          refs.pcScreen.current = null
          refs.pendingCandidates.current.screen = []
          const mainEl = mainVideoRef.current
          const remoteEl = remoteCamRef.current
          if (refs.remoteCamStream.current && mainEl)
            mainEl.srcObject = refs.remoteCamStream.current
          if (remoteEl) remoteEl.style.display = 'none'
        }
      }
    }

    return pc
  }

  async function renegotiateAll() {
    const oldCam = refs.pcCam.current
    if (oldCam) {
      oldCam.close()
      refs.pcCam.current = null
      refs.pendingCandidates.current.camOut = []
    }
    const localCam = refs.localCam.current
    if (localCam) {
      const pc = createPC('cam', 'send')
      refs.pcCam.current = pc
      localCam.getTracks().forEach((t) => pc.addTrack(t, localCam))
      const offer = await pc.createOffer()
      await pc.setLocalDescription(offer)
      send('offer', offer, 'cam')
    }

    if (role === 'presenter') {
      const oldScreen = refs.pcScreen.current
      if (oldScreen) {
        oldScreen.close()
        refs.pcScreen.current = null
        refs.pendingCandidates.current.screen = []
      }
      const localScreen = refs.localScreen.current
      if (refs.isSharingScreen.current && localScreen) {
        const pc = createPC('screen', 'send')
        refs.pcScreen.current = pc
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
      refs.localScreen.current = stream
      refs.isSharingScreen.current = true
      onSharingChange?.(true)

      if (mainVideoRef.current) mainVideoRef.current.srcObject = stream
      const remoteEl = remoteCamRef.current
      if (remoteEl) {
        remoteEl.srcObject = refs.remoteCamStream.current
        remoteEl.style.display = refs.remoteCamStream.current ? 'block' : 'none'
      }

      const pc = createPC('screen', 'send')
      refs.pcScreen.current = pc
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
    refs.localScreen.current?.getTracks().forEach((t) => t.stop())
    refs.localScreen.current = null
    if (refs.pcScreen.current) {
      refs.pcScreen.current.close()
      refs.pcScreen.current = null
    }
    stopScreenUI()
  }

  async function startCamera() {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true })
      refs.localCam.current = stream
      if (selfCamRef.current) selfCamRef.current.srcObject = stream
      if (refs.ws.current?.readyState === WebSocket.OPEN) {
        void renegotiateAll()
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to start camera')
    }
  }

  return { createPC, renegotiateAll, startScreen, stopScreen, startCamera }
}
