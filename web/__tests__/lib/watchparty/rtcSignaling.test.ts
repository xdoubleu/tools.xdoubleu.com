import { createSignalHandler } from '@/lib/watchparty/rtcSignaling'
import type { RTCRefs } from '@/lib/watchparty/rtcMedia'

class MockPC {
  connectionState = 'new'
  signalingState = 'stable'
  remoteDescription: RTCSessionDescriptionInit | null = null
  tracks: unknown[] = []
  addIceCandidate = jest.fn(async () => {})
  addTrack = jest.fn((t: unknown) => {
    this.tracks.push(t)
  })
  createOffer = jest.fn(async () => ({ type: 'offer', sdp: 'mock' }) as RTCSessionDescriptionInit)
  createAnswer = jest.fn(async () => ({ type: 'answer', sdp: 'mock' }) as RTCSessionDescriptionInit)
  setLocalDescription = jest.fn(async (d: RTCSessionDescriptionInit) => {
    this.signalingState = d.type === 'offer' ? 'have-local-offer' : 'stable'
  })
  setRemoteDescription = jest.fn(async (d: RTCSessionDescriptionInit) => {
    this.remoteDescription = d
  })
  close = jest.fn(() => {
    this.connectionState = 'closed'
  })
}

// Single funnel for the mock → browser-type casts the RTC interfaces require.
// eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
const asPC = (pc: MockPC) => pc as unknown as RTCPeerConnection
// eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
const asStream = (s: { getTracks: () => unknown[] }) => s as unknown as MediaStream

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

function makeHandler(role: 'presenter' | 'viewer' = 'viewer', onCreatePC?: (pc: MockPC) => void) {
  const refs = makeRefs()
  const send = jest.fn()
  const created: MockPC[] = []
  const createPC = jest.fn(() => {
    const pc = new MockPC()
    created.push(pc)
    onCreatePC?.(pc)
    return asPC(pc)
  })
  const handler = createSignalHandler({ refs, role, send, createPC })
  const dispatch = (msg: unknown) =>
    handler(new MessageEvent('message', { data: JSON.stringify(msg) }))
  return { refs, send, createPC, created, dispatch }
}

const sdpOffer: RTCSessionDescriptionInit = { type: 'offer', sdp: 'remote' }
const sdpAnswer: RTCSessionDescriptionInit = { type: 'answer', sdp: 'remote' }
const candidate = { candidate: 'candidate:0 1 UDP 1 192.168.0.1 9 typ host' }

describe('createSignalHandler — malformed input', () => {
  it('ignores non-signal messages', async () => {
    const { dispatch, createPC } = makeHandler()
    await dispatch({ hello: 'world' })
    await dispatch({ type: 'unknown', payload: {}, trackType: 'cam' })
    expect(createPC).not.toHaveBeenCalled()
  })

  it('ignores an offer whose payload is not an SDP', async () => {
    const { dispatch, createPC } = makeHandler()
    await dispatch({ type: 'offer', trackType: 'cam', payload: candidate })
    expect(createPC).not.toHaveBeenCalled()
  })

  it('ignores a candidate whose payload is an SDP', async () => {
    const { dispatch, refs } = makeHandler()
    await dispatch({ type: 'candidate', trackType: 'cam', direction: 'send', payload: sdpOffer })
    expect(refs.pendingCandidates.current.camIn).toHaveLength(0)
  })
})

describe('createSignalHandler — offers', () => {
  it('answers a cam offer with a recv connection and flushes queued candidates', async () => {
    // Queue a candidate while the remote description is being applied — the
    // handler clears the queue when the offer arrives, so only candidates
    // racing the SDP exchange are flushed.
    const handler = makeHandler('viewer', (pc) => {
      pc.setRemoteDescription.mockImplementation(async (d: RTCSessionDescriptionInit) => {
        pc.remoteDescription = d
        refs.pendingCandidates.current.camIn.push(candidate)
      })
    })
    const { dispatch, refs, send, createPC, created } = handler

    await dispatch({ type: 'offer', trackType: 'cam', payload: sdpOffer })

    expect(createPC).toHaveBeenCalledWith('cam', 'recv')
    const pc = created[0]
    expect(refs.pcInCam.current).toBe(asPC(pc))
    expect(pc.setRemoteDescription).toHaveBeenCalledWith(sdpOffer)
    expect(pc.addIceCandidate).toHaveBeenCalledWith(candidate)
    expect(refs.pendingCandidates.current.camIn).toHaveLength(0)
    expect(send).toHaveBeenCalledWith('answer', expect.objectContaining({ type: 'answer' }), 'cam')
  })

  it('closes the previous incoming cam connection when a new offer arrives', async () => {
    const { dispatch, refs } = makeHandler()
    const old = new MockPC()
    refs.pcInCam.current = asPC(old)

    await dispatch({ type: 'offer', trackType: 'cam', payload: sdpOffer })
    expect(old.close).toHaveBeenCalled()
  })

  it('answers a screen offer on the screen connection', async () => {
    const handler = makeHandler('viewer', (pc) => {
      pc.setRemoteDescription.mockImplementation(async (d: RTCSessionDescriptionInit) => {
        pc.remoteDescription = d
        refs.pendingCandidates.current.screen.push(candidate)
      })
    })
    const { dispatch, refs, send, createPC, created } = handler

    await dispatch({ type: 'offer', trackType: 'screen', payload: sdpOffer })

    expect(createPC).toHaveBeenCalledWith('screen', 'recv')
    expect(refs.pcScreen.current).toBe(asPC(created[0]))
    expect(created[0].addIceCandidate).toHaveBeenCalledWith(candidate)
    expect(send).toHaveBeenCalledWith(
      'answer',
      expect.objectContaining({ type: 'answer' }),
      'screen'
    )
  })

  it('abandons a superseded cam offer after the remote description settles', async () => {
    const { dispatch, refs, send, created } = makeHandler()
    const replacement = new MockPC()

    const pending = dispatch({ type: 'offer', trackType: 'cam', payload: sdpOffer })
    // Simulate a newer offer replacing the connection while the first awaits.
    refs.pcInCam.current = asPC(replacement)
    await pending

    expect(created[0].createAnswer).not.toHaveBeenCalled()
    expect(send).not.toHaveBeenCalled()
  })
})

describe('createSignalHandler — answers', () => {
  it('applies a cam answer and flushes outgoing queued candidates', async () => {
    const { dispatch, refs } = makeHandler()
    const pc = new MockPC()
    pc.signalingState = 'have-local-offer'
    refs.pcCam.current = asPC(pc)
    refs.pendingCandidates.current.camOut = [candidate]

    await dispatch({ type: 'answer', trackType: 'cam', payload: sdpAnswer })

    expect(pc.setRemoteDescription).toHaveBeenCalledWith(sdpAnswer)
    expect(pc.addIceCandidate).toHaveBeenCalledWith(candidate)
    expect(refs.pendingCandidates.current.camOut).toHaveLength(0)
  })

  it('ignores a cam answer when no offer is outstanding', async () => {
    const { dispatch, refs } = makeHandler()
    const pc = new MockPC()
    refs.pcCam.current = asPC(pc)

    await dispatch({ type: 'answer', trackType: 'cam', payload: sdpAnswer })
    expect(pc.setRemoteDescription).not.toHaveBeenCalled()
  })

  it('applies a screen answer to an outstanding screen offer', async () => {
    const { dispatch, refs } = makeHandler()
    const pc = new MockPC()
    pc.signalingState = 'have-local-offer'
    refs.pcScreen.current = asPC(pc)

    await dispatch({ type: 'answer', trackType: 'screen', payload: sdpAnswer })
    expect(pc.setRemoteDescription).toHaveBeenCalledWith(sdpAnswer)
  })

  it('re-offers the screen when a presenter gets a stale screen answer mid-share', async () => {
    const { dispatch, refs, send, createPC } = makeHandler('presenter')
    const stale = new MockPC()
    refs.pcScreen.current = asPC(stale)
    refs.isSharingScreen.current = true
    refs.localScreen.current = asStream({ getTracks: () => [{ kind: 'video' }] })

    await dispatch({ type: 'answer', trackType: 'screen', payload: sdpAnswer })

    expect(stale.close).toHaveBeenCalled()
    expect(createPC).toHaveBeenCalledWith('screen', 'send')
    expect(send).toHaveBeenCalledWith('offer', expect.objectContaining({ type: 'offer' }), 'screen')
  })
})

describe('createSignalHandler — candidates', () => {
  it('adds an incoming cam candidate when the remote description is set', async () => {
    const { dispatch, refs } = makeHandler()
    const pc = new MockPC()
    pc.remoteDescription = sdpOffer
    refs.pcInCam.current = asPC(pc)

    await dispatch({ type: 'candidate', trackType: 'cam', direction: 'send', payload: candidate })
    expect(pc.addIceCandidate).toHaveBeenCalledWith(candidate)
  })

  it('queues an incoming cam candidate that arrives before the offer', async () => {
    const { dispatch, refs } = makeHandler()
    await dispatch({ type: 'candidate', trackType: 'cam', direction: 'send', payload: candidate })
    expect(refs.pendingCandidates.current.camIn).toEqual([candidate])
  })

  it('queues an outgoing-leg cam candidate until the answer lands', async () => {
    const { dispatch, refs } = makeHandler()
    const pc = new MockPC()
    refs.pcCam.current = asPC(pc)

    await dispatch({ type: 'candidate', trackType: 'cam', payload: candidate })
    expect(refs.pendingCandidates.current.camOut).toEqual([candidate])
  })

  it('adds a screen candidate when ready and queues it when not', async () => {
    const { dispatch, refs } = makeHandler()
    const pc = new MockPC()
    refs.pcScreen.current = asPC(pc)

    await dispatch({ type: 'candidate', trackType: 'screen', payload: candidate })
    expect(refs.pendingCandidates.current.screen).toEqual([candidate])

    pc.remoteDescription = sdpOffer
    await dispatch({ type: 'candidate', trackType: 'screen', payload: candidate })
    expect(pc.addIceCandidate).toHaveBeenCalledWith(candidate)
  })

  it('drops a screen candidate when no screen connection exists', async () => {
    const { dispatch, refs } = makeHandler()
    await dispatch({ type: 'candidate', trackType: 'screen', payload: candidate })
    expect(refs.pendingCandidates.current.screen).toHaveLength(0)
  })
})
