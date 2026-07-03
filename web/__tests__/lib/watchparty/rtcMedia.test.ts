import { createMediaController } from '@/lib/watchparty/rtcMedia'
import type { MediaControllerDeps, RTCRefs } from '@/lib/watchparty/rtcMedia'

class MockPC {
  connectionState = 'new'
  signalingState = 'stable'
  remoteDescription: RTCSessionDescriptionInit | null = null
  onicecandidate: ((e: { candidate: unknown }) => void) | null = null
  ontrack: ((e: { streams: unknown[] }) => void) | null = null
  onconnectionstatechange: (() => void) | null = null
  tracks: unknown[] = []
  addTrack = jest.fn((t: unknown) => {
    this.tracks.push(t)
  })
  createOffer = jest.fn(async () => ({ type: 'offer', sdp: 'mock' }) as RTCSessionDescriptionInit)
  createAnswer = jest.fn(async () => ({ type: 'answer', sdp: 'mock' }) as RTCSessionDescriptionInit)
  setLocalDescription = jest.fn(async () => {})
  setRemoteDescription = jest.fn(async () => {})
  addIceCandidate = jest.fn(async () => {})
  close = jest.fn(() => {
    this.connectionState = 'closed'
  })
}

type MockStream = ReturnType<typeof makeStream>

// Single funnel for the mock → browser-type casts the RTC interfaces require.
// eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
const asPC = (pc: MockPC) => pc as unknown as RTCPeerConnection
// eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
const asStream = (s: MockStream) => s as unknown as MediaStream
const asVideoEl = (el: { srcObject: unknown; style: { display: string } }) =>
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
  el as unknown as HTMLVideoElement
// eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
const asWS = (ws: { readyState: number }) => ws as unknown as WebSocket

let createdPCs: MockPC[]

function makeVideoEl() {
  return asVideoEl({ srcObject: null, style: { display: '' } })
}

function makeTrack(kind: 'audio' | 'video') {
  return { kind, enabled: true, stop: jest.fn(), onended: null as (() => void) | null }
}

function makeStream() {
  const video = makeTrack('video')
  const audio = makeTrack('audio')
  return {
    getTracks: () => [audio, video],
    getAudioTracks: () => [audio],
    getVideoTracks: () => [video],
    _video: video,
    _audio: audio
  }
}

function makeRefs(): RTCRefs {
  return {
    ws: { current: null },
    pcCam: { current: null },
    pcInCam: { current: null },
    pcScreen: { current: null },
    localCam: { current: null },
    localScreen: { current: null },
    remoteCamStream: { current: null },
    isSharingScreen: { current: false },
    pendingCandidates: { current: { camOut: [], camIn: [], screen: [] } }
  }
}

function makeController(role: 'presenter' | 'viewer' = 'viewer') {
  const refs = makeRefs()
  const send = jest.fn()
  const setError = jest.fn()
  const onSharingChange = jest.fn()
  const mainVideoRef = { current: makeVideoEl() }
  const selfCamRef = { current: makeVideoEl() }
  const remoteCamRef = { current: makeVideoEl() }
  const deps: MediaControllerDeps = {
    refs,
    role,
    send,
    mainVideoRef,
    selfCamRef,
    remoteCamRef,
    onSharingChange,
    setError
  }
  const controller = createMediaController(deps)
  const createMockPC = (trackType: 'cam' | 'screen', direction?: 'send' | 'recv') => {
    controller.createPC(trackType, direction)
    return createdPCs[createdPCs.length - 1]
  }
  return {
    refs,
    send,
    setError,
    onSharingChange,
    mainVideoRef,
    selfCamRef,
    remoteCamRef,
    controller,
    createMockPC
  }
}

beforeEach(() => {
  createdPCs = []
  Object.defineProperty(global, 'RTCPeerConnection', {
    value: jest.fn(() => {
      const pc = new MockPC()
      createdPCs.push(pc)
      return pc
    }),
    writable: true,
    configurable: true
  })
  Object.defineProperty(global.navigator, 'mediaDevices', {
    value: {
      getUserMedia: jest.fn().mockResolvedValue(asStream(makeStream())),
      getDisplayMedia: jest.fn().mockResolvedValue(asStream(makeStream()))
    },
    writable: true,
    configurable: true
  })
})

afterEach(() => {
  jest.clearAllMocks()
})

describe('createPC', () => {
  it('forwards ICE candidates through send', () => {
    const { send, createMockPC } = makeController()
    const pc = createMockPC('cam', 'send')
    pc.onicecandidate?.({ candidate: { candidate: 'c' } })
    expect(send).toHaveBeenCalledWith('candidate', { candidate: 'c' }, 'cam', 'send')
    pc.onicecandidate?.({ candidate: null })
    expect(send).toHaveBeenCalledTimes(1)
  })

  it('shows an incoming cam full-screen when no screen share is active', () => {
    const { refs, mainVideoRef, remoteCamRef, createMockPC } = makeController()
    const pc = createMockPC('cam', 'recv')
    const stream = asStream(makeStream())
    pc.ontrack?.({ streams: [stream] })
    expect(refs.remoteCamStream.current).toBe(stream)
    expect(mainVideoRef.current.srcObject).toBe(stream)
    expect(remoteCamRef.current.style.display).toBe('none')
  })

  it('shows an incoming cam as overlay while a screen share is active', () => {
    const { refs, remoteCamRef, createMockPC } = makeController()
    refs.isSharingScreen.current = true
    const pc = createMockPC('cam', 'recv')
    const stream = asStream(makeStream())
    pc.ontrack?.({ streams: [stream] })
    expect(remoteCamRef.current.srcObject).toBe(stream)
    expect(remoteCamRef.current.style.display).toBe('block')
  })

  it('promotes an incoming screen track to the main video', () => {
    const { refs, mainVideoRef, remoteCamRef, onSharingChange, createMockPC } =
      makeController('presenter')
    const camStream = asStream(makeStream())
    refs.remoteCamStream.current = camStream
    const pc = createMockPC('screen', 'recv')
    const screenStream = asStream(makeStream())
    pc.ontrack?.({ streams: [screenStream] })
    expect(refs.isSharingScreen.current).toBe(true)
    expect(onSharingChange).toHaveBeenCalledWith(true)
    expect(mainVideoRef.current.srcObject).toBe(screenStream)
    expect(remoteCamRef.current.srcObject).toBe(camStream)
    expect(remoteCamRef.current.style.display).toBe('block')
  })

  it('clears the incoming cam connection when it fails', () => {
    const { refs, createMockPC } = makeController()
    const pc = createMockPC('cam', 'recv')
    refs.pcInCam.current = asPC(pc)
    refs.pendingCandidates.current.camIn = [{ candidate: 'x' }]
    pc.connectionState = 'failed'
    pc.onconnectionstatechange?.()
    expect(refs.pcInCam.current).toBeNull()
    expect(refs.pendingCandidates.current.camIn).toHaveLength(0)
  })

  it('re-offers the outgoing cam when it fails', async () => {
    const { refs, send, createMockPC } = makeController()
    refs.localCam.current = asStream(makeStream())
    const pc = createMockPC('cam', 'send')
    refs.pcCam.current = asPC(pc)
    pc.connectionState = 'failed'
    pc.onconnectionstatechange?.()
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(refs.pcCam.current).not.toBe(asPC(pc))
    expect(send).toHaveBeenCalledWith('offer', expect.objectContaining({ type: 'offer' }), 'cam')
  })

  it('restores the cam view when the presenter screen connection drops', () => {
    const { refs, mainVideoRef, remoteCamRef, onSharingChange, createMockPC } =
      makeController('presenter')
    const camStream = asStream(makeStream())
    refs.remoteCamStream.current = camStream
    refs.isSharingScreen.current = true
    const pc = createMockPC('screen', 'send')
    pc.connectionState = 'disconnected'
    pc.onconnectionstatechange?.()
    expect(refs.isSharingScreen.current).toBe(false)
    expect(onSharingChange).toHaveBeenCalledWith(false)
    expect(mainVideoRef.current.srcObject).toBe(camStream)
    expect(remoteCamRef.current.style.display).toBe('none')
  })

  it('resets viewer screen state when the screen connection fails', () => {
    const { refs, mainVideoRef, createMockPC } = makeController('viewer')
    const camStream = asStream(makeStream())
    refs.remoteCamStream.current = camStream
    refs.isSharingScreen.current = true
    const pc = createMockPC('screen', 'recv')
    refs.pcScreen.current = asPC(pc)
    pc.connectionState = 'failed'
    pc.onconnectionstatechange?.()
    expect(refs.isSharingScreen.current).toBe(false)
    expect(refs.pcScreen.current).toBeNull()
    expect(mainVideoRef.current.srcObject).toBe(camStream)
  })
})

describe('renegotiateAll', () => {
  it('replaces the outgoing cam connection and re-offers', async () => {
    const { controller, refs, send } = makeController()
    const old = new MockPC()
    refs.pcCam.current = asPC(old)
    refs.localCam.current = asStream(makeStream())

    await controller.renegotiateAll()

    expect(old.close).toHaveBeenCalled()
    expect(refs.pcCam.current).not.toBe(asPC(old))
    expect(send).toHaveBeenCalledWith('offer', expect.objectContaining({ type: 'offer' }), 'cam')
  })

  it('also re-offers the screen for a sharing presenter', async () => {
    const { controller, refs, send } = makeController('presenter')
    refs.localCam.current = asStream(makeStream())
    refs.localScreen.current = asStream(makeStream())
    refs.isSharingScreen.current = true

    await controller.renegotiateAll()

    expect(send).toHaveBeenCalledWith('offer', expect.objectContaining({ type: 'offer' }), 'cam')
    expect(send).toHaveBeenCalledWith('offer', expect.objectContaining({ type: 'offer' }), 'screen')
  })

  it('does nothing without local media', async () => {
    const { controller, send } = makeController()
    await controller.renegotiateAll()
    expect(send).not.toHaveBeenCalled()
  })
})

describe('startScreen / stopScreen', () => {
  it('captures the display, shows it locally, and offers it', async () => {
    const { controller, refs, send, mainVideoRef, onSharingChange } = makeController('presenter')
    await controller.startScreen()

    expect(refs.isSharingScreen.current).toBe(true)
    expect(onSharingChange).toHaveBeenCalledWith(true)
    expect(mainVideoRef.current.srcObject).toBe(refs.localScreen.current)
    expect(send).toHaveBeenCalledWith('offer', expect.objectContaining({ type: 'offer' }), 'screen')
  })

  it('reports an error when display capture is refused', async () => {
    jest
      .mocked(navigator.mediaDevices.getDisplayMedia)
      .mockRejectedValueOnce(new Error('Permission denied'))
    const { controller, setError } = makeController('presenter')
    await controller.startScreen()
    expect(setError).toHaveBeenCalledWith('Permission denied')
  })

  it('stopScreen stops tracks, closes the connection, and restores the cam', async () => {
    const screenStream = makeStream()
    jest
      .mocked(navigator.mediaDevices.getDisplayMedia)
      .mockResolvedValueOnce(asStream(screenStream))
    const { controller, refs, mainVideoRef, onSharingChange } = makeController('presenter')
    const camStream = asStream(makeStream())
    refs.remoteCamStream.current = camStream
    await controller.startScreen()
    const pc = createdPCs[createdPCs.length - 1]

    controller.stopScreen()

    expect(screenStream._video.stop).toHaveBeenCalled()
    expect(pc.close).toHaveBeenCalled()
    expect(refs.localScreen.current).toBeNull()
    expect(refs.pcScreen.current).toBeNull()
    expect(onSharingChange).toHaveBeenLastCalledWith(false)
    expect(mainVideoRef.current.srcObject).toBe(camStream)
  })

  it('ends the share when the captured track ends (browser stop button)', async () => {
    const screenStream = makeStream()
    jest
      .mocked(navigator.mediaDevices.getDisplayMedia)
      .mockResolvedValueOnce(asStream(screenStream))
    const { controller, refs } = makeController('presenter')
    await controller.startScreen()

    screenStream._video.onended?.()
    expect(refs.isSharingScreen.current).toBe(false)
    expect(refs.localScreen.current).toBeNull()
  })
})

describe('startCamera', () => {
  it('captures the camera and previews it in the self-cam element', async () => {
    const { controller, refs, selfCamRef, send } = makeController()
    await controller.startCamera()
    expect(refs.localCam.current).not.toBeNull()
    expect(selfCamRef.current.srcObject).toBe(refs.localCam.current)
    // No open socket yet, so no renegotiation offer is sent.
    expect(send).not.toHaveBeenCalled()
  })

  it('renegotiates immediately when the socket is already open', async () => {
    const { controller, refs, send } = makeController()
    refs.ws.current = asWS({ readyState: WebSocket.OPEN })
    await controller.startCamera()
    // Renegotiation is fired without being awaited; let its microtasks settle.
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(send).toHaveBeenCalledWith('offer', expect.objectContaining({ type: 'offer' }), 'cam')
  })

  it('reports an error when camera capture is refused', async () => {
    jest.mocked(navigator.mediaDevices.getUserMedia).mockRejectedValueOnce(new Error('No camera'))
    const { controller, setError } = makeController()
    await controller.startCamera()
    expect(setError).toHaveBeenCalledWith('No camera')
  })
})
