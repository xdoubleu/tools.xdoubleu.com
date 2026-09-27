'use client'

import { useRef } from 'react'
import { useWatchPartyRTC } from '@/hooks/useWatchPartyRTC'
import WatchPartyRoom from '@/components/watchparty/WatchPartyRoom'

export default function ViewerClient({ id }: { id: string }) {
  const mainVideoRef = useRef<HTMLVideoElement>(null)
  const selfCamRef = useRef<HTMLVideoElement>(null)
  const remoteCamRef = useRef<HTMLVideoElement>(null)

  const rtc = useWatchPartyRTC({
    id,
    role: 'viewer',
    mainVideoRef,
    selfCamRef,
    remoteCamRef
  })

  return (
    <WatchPartyRoom
      id={id}
      crumb="Room"
      {...rtc}
      mainVideoRef={mainVideoRef}
      selfCamRef={selfCamRef}
      remoteCamRef={remoteCamRef}
    />
  )
}
