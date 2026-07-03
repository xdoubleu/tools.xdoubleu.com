import type { TrackType, WsMessage } from '@/lib/watchparty/types'
import type { RTCRefs, SendFn } from '@/lib/watchparty/rtcMedia'

function isWsMessage(value: unknown): value is WsMessage {
  return (
    value !== null &&
    typeof value === 'object' &&
    'type' in value &&
    'payload' in value &&
    'trackType' in value &&
    (value.type === 'offer' || value.type === 'answer' || value.type === 'candidate')
  )
}

function isRTCSDP(
  payload: RTCSessionDescriptionInit | RTCIceCandidateInit
): payload is RTCSessionDescriptionInit {
  return (
    'type' in payload &&
    (payload.type === 'offer' ||
      payload.type === 'answer' ||
      payload.type === 'pranswer' ||
      payload.type === 'rollback')
  )
}

function isRTCCandidate(
  payload: RTCSessionDescriptionInit | RTCIceCandidateInit
): payload is RTCIceCandidateInit {
  return !isRTCSDP(payload)
}

export interface SignalHandlerDeps {
  refs: RTCRefs
  role: 'presenter' | 'viewer'
  send: SendFn
  createPC: (trackType: TrackType, direction?: 'send' | 'recv') => RTCPeerConnection
}

// createSignalHandler returns the WebSocket onmessage handler: it demuxes
// offer/answer/candidate messages per track type, queueing ICE candidates that
// arrive before the matching remote description is set.
export function createSignalHandler(deps: SignalHandlerDeps) {
  const { refs, role, send, createPC } = deps

  return async function onSignal(event: MessageEvent<string>) {
    const parsed: unknown = JSON.parse(event.data)
    if (!isWsMessage(parsed)) return
    const msg = parsed
    const tt = msg.trackType

    if (msg.type === 'offer') {
      if (!isRTCSDP(msg.payload)) return
      if (tt === 'cam') {
        if (refs.pcInCam.current) refs.pcInCam.current.close()
        refs.pendingCandidates.current.camIn = []
        const pc = createPC('cam', 'recv')
        refs.pcInCam.current = pc

        await pc.setRemoteDescription(msg.payload)
        if (refs.pcInCam.current !== pc) return
        for (const c of refs.pendingCandidates.current.camIn.splice(0)) await pc.addIceCandidate(c)
        if (refs.pcInCam.current !== pc) return
        const answer = await pc.createAnswer()
        if (refs.pcInCam.current !== pc) return
        await pc.setLocalDescription(answer)
        if (refs.pcInCam.current !== pc) return
        send('answer', answer, tt)
      } else {
        if (refs.pcScreen.current) refs.pcScreen.current.close()
        refs.pendingCandidates.current.screen = []
        const pcScreen = createPC(tt, 'recv')
        refs.pcScreen.current = pcScreen

        await pcScreen.setRemoteDescription(msg.payload)
        if (refs.pcScreen.current !== pcScreen) return
        for (const c of refs.pendingCandidates.current[tt].splice(0))
          await pcScreen.addIceCandidate(c)
        if (refs.pcScreen.current !== pcScreen) return
        const answer = await pcScreen.createAnswer()
        if (refs.pcScreen.current !== pcScreen) return
        await pcScreen.setLocalDescription(answer)
        if (refs.pcScreen.current !== pcScreen) return
        send('answer', answer, tt)
      }
    }

    if (msg.type === 'answer') {
      if (!isRTCSDP(msg.payload)) return
      if (tt === 'cam') {
        const pc = refs.pcCam.current
        if (pc && pc.signalingState === 'have-local-offer') {
          await pc.setRemoteDescription(msg.payload)
          for (const c of refs.pendingCandidates.current.camOut.splice(0))
            await pc.addIceCandidate(c)
        }
      } else {
        const pc = refs.pcScreen.current
        if (pc && pc.signalingState === 'have-local-offer') {
          await pc.setRemoteDescription(msg.payload)
          for (const c of refs.pendingCandidates.current[tt].splice(0)) await pc.addIceCandidate(c)
        } else if (
          role === 'presenter' &&
          refs.isSharingScreen.current &&
          refs.localScreen.current
        ) {
          if (refs.pcScreen.current) {
            refs.pcScreen.current.close()
            refs.pcScreen.current = null
          }
          refs.pendingCandidates.current.screen = []
          const newPc = createPC('screen', 'send')
          refs.pcScreen.current = newPc
          refs.localScreen.current
            .getTracks()
            .forEach((t) => newPc.addTrack(t, refs.localScreen.current!))
          const offer = await newPc.createOffer()
          await newPc.setLocalDescription(offer)
          send('offer', offer, 'screen')
        }
      }
    }

    if (msg.type === 'candidate') {
      if (!isRTCCandidate(msg.payload)) return
      if (tt === 'cam') {
        if (msg.direction === 'send') {
          const inCam = refs.pcInCam.current
          if (inCam && inCam.remoteDescription) {
            await inCam.addIceCandidate(msg.payload)
          } else {
            refs.pendingCandidates.current.camIn.push(msg.payload)
          }
        } else {
          const outCam = refs.pcCam.current
          if (outCam && outCam.remoteDescription) {
            await outCam.addIceCandidate(msg.payload)
          } else {
            refs.pendingCandidates.current.camOut.push(msg.payload)
          }
        }
      } else {
        const pc = refs.pcScreen.current
        if (pc && pc.remoteDescription) {
          await pc.addIceCandidate(msg.payload)
        } else if (pc) {
          refs.pendingCandidates.current[tt].push(msg.payload)
        }
      }
    }
  }
}
