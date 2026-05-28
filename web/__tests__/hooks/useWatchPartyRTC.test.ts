import { renderHook, act, waitFor } from '@testing-library/react'

jest.mock('@/lib/env', () => ({ getApiUrl: jest.fn(() => 'http://localhost:8000') }))
jest.mock('@/lib/watchparty/roomUtils', () => ({
  buildWsUrl: jest.fn(() => 'ws://localhost:8000/ws')
}))

// ── WebSocket mock ─────────────────────────────────────────────────────────────

class MockWebSocket {
  static OPEN = 1
  readyState = 0
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onerror: ((e: Event) => void) | null = null
  onmessage: ((e: MessageEvent) => void) | null = null
  sent: string[] = []

  send(data: string) {
    this.sent.push(data)
  }
  close() {
    this.readyState = 3
    this.onclose?.()
  }
  simulateOpen() {
    this.readyState = 1
    this.onopen?.()
  }
  simulateMessage(data: unknown) {
    this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(data) }))
  }
}

let mockWs: MockWebSocket
let mockWebSocketFn: jest.Mock
let mockRTCPeerConnectionFn: jest.Mock

// ── RTCPeerConnection mock ─────────────────────────────────────────────────────

class MockPC {
  connectionState = 'new'
  signalingState = 'stable'
  remoteDescription: RTCSessionDescriptionInit | null = null
  onicecandidate: ((e: RTCPeerConnectionIceEvent) => void) | null = null
  ontrack: ((e: RTCTrackEvent) => void) | null = null
  onconnectionstatechange: (() => void) | null = null
  tracks: MediaStreamTrack[] = []

  addTrack(t: MediaStreamTrack) {
    this.tracks.push(t)
  }
  async createOffer() {
    return { type: 'offer', sdp: 'mock-sdp' } as RTCSessionDescriptionInit
  }
  async createAnswer() {
    return { type: 'answer', sdp: 'mock-sdp' } as RTCSessionDescriptionInit
  }
  async setLocalDescription(d: RTCSessionDescriptionInit) {
    this.signalingState = d.type === 'offer' ? 'have-local-offer' : 'stable'
  }
  async setRemoteDescription(d: RTCSessionDescriptionInit) {
    this.remoteDescription = d
  }
  async addIceCandidate() {}
  close() {
    this.connectionState = 'closed'
  }
}

// ── MediaStream / track mocks ──────────────────────────────────────────────────

function makeMockTrack(kind: 'audio' | 'video') {
  return { kind, enabled: true, stop: jest.fn(), onended: null as (() => void) | null }
}

function makeMockStream() {
  const audioTrack = makeMockTrack('audio')
  const videoTrack = makeMockTrack('video')
  return {
    getTracks: jest.fn(() => [audioTrack, videoTrack]),
    getAudioTracks: jest.fn(() => [audioTrack]),
    getVideoTracks: jest.fn(() => [videoTrack]),
    _audio: audioTrack,
    _video: videoTrack
  }
}

// ── Setup ──────────────────────────────────────────────────────────────────────

beforeEach(() => {
  jest.useFakeTimers()

  mockWebSocketFn = jest.fn().mockImplementation(() => {
    mockWs = new MockWebSocket()
    return mockWs
  })
  Object.defineProperty(mockWebSocketFn, 'OPEN', { value: 1, configurable: true })
  Object.defineProperty(global, 'WebSocket', {
    value: mockWebSocketFn,
    writable: true,
    configurable: true
  })

  mockRTCPeerConnectionFn = jest.fn(() => new MockPC())
  Object.defineProperty(global, 'RTCPeerConnection', {
    value: mockRTCPeerConnectionFn,
    writable: true,
    configurable: true
  })

  const mockStream = makeMockStream()
  Object.defineProperty(global.navigator, 'mediaDevices', {
    value: {
      getUserMedia: jest.fn().mockResolvedValue(mockStream),
      getDisplayMedia: jest.fn().mockResolvedValue(mockStream)
    },
    writable: true,
    configurable: true
  })
})

afterEach(() => {
  jest.useRealTimers()
  jest.clearAllMocks()
})

// ── Import hook after mocks ───────────────────────────────────────────────────

import { useWatchPartyRTC } from '@/hooks/useWatchPartyRTC'

// ── Tests ──────────────────────────────────────────────────────────────────────

function makeRefs() {
  const mainVideoRef = { current: null as HTMLVideoElement | null }
  const selfCamRef = { current: null as HTMLVideoElement | null }
  const remoteCamRef = { current: null as HTMLVideoElement | null }
  return { mainVideoRef, selfCamRef, remoteCamRef }
}

describe('useWatchPartyRTC — initial state', () => {
  it('starts with connecting status and defaults', () => {
    const { result } = renderHook(() =>
      useWatchPartyRTC({ id: 'room1', role: 'viewer', ...makeRefs() })
    )
    expect(result.current.status).toBe('connecting')
    expect(result.current.micEnabled).toBe(true)
    expect(result.current.camEnabled).toBe(true)
    expect(result.current.selfCamVisible).toBe(true)
    expect(result.current.error).toBeNull()
  })

  it('creates a WebSocket on mount', () => {
    renderHook(() => useWatchPartyRTC({ id: 'room1', role: 'viewer', ...makeRefs() }))
    expect(global.WebSocket).toHaveBeenCalledWith('ws://localhost:8000/ws')
  })

  it('calls getUserMedia on mount', () => {
    renderHook(() => useWatchPartyRTC({ id: 'room1', role: 'viewer', ...makeRefs() }))
    expect(navigator.mediaDevices.getUserMedia).toHaveBeenCalledWith({
      video: true,
      audio: true
    })
  })
})

describe('useWatchPartyRTC — status transitions', () => {
  it('becomes connected when WebSocket opens', async () => {
    const { result } = renderHook(() =>
      useWatchPartyRTC({ id: 'room1', role: 'viewer', ...makeRefs() })
    )
    act(() => {
      mockWs.simulateOpen()
    })
    await waitFor(() => expect(result.current.status).toBe('connected'))
  })

  it('becomes disconnected when WebSocket closes', async () => {
    const { result } = renderHook(() =>
      useWatchPartyRTC({ id: 'room1', role: 'viewer', ...makeRefs() })
    )
    act(() => {
      mockWs.simulateOpen()
    })
    await waitFor(() => expect(result.current.status).toBe('connected'))

    act(() => {
      mockWs.close()
    })
    await waitFor(() => expect(result.current.status).toBe('disconnected'))
  })

  it('schedules reconnect after disconnect', async () => {
    renderHook(() => useWatchPartyRTC({ id: 'room1', role: 'viewer', ...makeRefs() }))
    act(() => {
      mockWs.simulateOpen()
      mockWs.close()
    })
    const callsBefore = mockWebSocketFn.mock.calls.length
    act(() => {
      jest.advanceTimersByTime(2000)
    })
    expect(mockWebSocketFn.mock.calls.length).toBeGreaterThan(callsBefore)
  })
})

describe('useWatchPartyRTC — toggle controls', () => {
  it('toggleMic flips micEnabled', () => {
    const { result } = renderHook(() =>
      useWatchPartyRTC({ id: 'room1', role: 'viewer', ...makeRefs() })
    )
    act(() => result.current.toggleMic())
    expect(result.current.micEnabled).toBe(false)
    act(() => result.current.toggleMic())
    expect(result.current.micEnabled).toBe(true)
  })

  it('toggleCam flips camEnabled', () => {
    const { result } = renderHook(() =>
      useWatchPartyRTC({ id: 'room1', role: 'viewer', ...makeRefs() })
    )
    act(() => result.current.toggleCam())
    expect(result.current.camEnabled).toBe(false)
  })

  it('toggleSelfCam flips selfCamVisible', () => {
    const { result } = renderHook(() =>
      useWatchPartyRTC({ id: 'room1', role: 'viewer', ...makeRefs() })
    )
    act(() => result.current.toggleSelfCam())
    expect(result.current.selfCamVisible).toBe(false)
  })
})

describe('useWatchPartyRTC — getUserMedia error', () => {
  it('sets error when getUserMedia rejects', async () => {
    jest
      .mocked(navigator.mediaDevices.getUserMedia)
      .mockRejectedValueOnce(new Error('Permission denied'))
    const { result } = renderHook(() =>
      useWatchPartyRTC({ id: 'room1', role: 'viewer', ...makeRefs() })
    )
    await waitFor(() => expect(result.current.error).toBe('Permission denied'))
  })
})

describe('useWatchPartyRTC — WS message handling', () => {
  it('handles cam offer message without throwing', async () => {
    const { result } = renderHook(() =>
      useWatchPartyRTC({ id: 'room1', role: 'viewer', ...makeRefs() })
    )
    act(() => mockWs.simulateOpen())
    await waitFor(() => expect(result.current.status).toBe('connected'))

    await act(async () => {
      mockWs.simulateMessage({
        type: 'offer',
        trackType: 'cam',
        payload: { type: 'offer', sdp: 'mock' }
      })
      await Promise.resolve()
    })
    expect(global.RTCPeerConnection).toHaveBeenCalled()
  })

  it('handles candidate message without throwing', async () => {
    const { result } = renderHook(() =>
      useWatchPartyRTC({ id: 'room1', role: 'viewer', ...makeRefs() })
    )
    act(() => mockWs.simulateOpen())
    await waitFor(() => expect(result.current.status).toBe('connected'))

    await act(async () => {
      mockWs.simulateMessage({
        type: 'candidate',
        trackType: 'cam',
        direction: 'send',
        payload: { candidate: 'candidate:0 1 UDP 2130706431 192.168.0.1 9 typ host' }
      })
    })
    expect(result.current.error).toBeNull()
  })
})

describe('useWatchPartyRTC — cleanup', () => {
  it('closes WebSocket on unmount without throwing', () => {
    const { unmount } = renderHook(() =>
      useWatchPartyRTC({ id: 'room1', role: 'viewer', ...makeRefs() })
    )
    act(() => mockWs.simulateOpen())
    expect(() => unmount()).not.toThrow()
  })
})
