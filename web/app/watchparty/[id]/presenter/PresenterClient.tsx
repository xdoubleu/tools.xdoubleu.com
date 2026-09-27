'use client'

import { useRef, useState } from 'react'
import { useWatchPartyRTC, type ScreenControls } from '@/hooks/useWatchPartyRTC'
import { Button } from '@/components/ui/button'
import WatchPartyRoom from '@/components/watchparty/WatchPartyRoom'

export default function PresenterClient({ id }: { id: string }) {
  const mainVideoRef = useRef<HTMLVideoElement>(null)
  const selfCamRef = useRef<HTMLVideoElement>(null)
  const remoteCamRef = useRef<HTMLVideoElement>(null)
  const screenControlsRef = useRef<ScreenControls>({
    start: () => Promise.resolve(),
    stop: () => {}
  })

  const [sharing, setSharing] = useState(false)

  const rtc = useWatchPartyRTC({
    id,
    role: 'presenter',
    mainVideoRef,
    selfCamRef,
    remoteCamRef,
    onSharingChange: setSharing,
    screenControls: screenControlsRef
  })

  return (
    <WatchPartyRoom
      id={id}
      crumb="Presenter"
      {...rtc}
      mainVideoRef={mainVideoRef}
      selfCamRef={selfCamRef}
      remoteCamRef={remoteCamRef}
      mainMuted
      controls={
        sharing ? (
          <Button variant="destructive" onClick={() => screenControlsRef.current.stop()}>
            Stop Sharing
          </Button>
        ) : (
          <Button
            onClick={() => void screenControlsRef.current.start()}
            disabled={rtc.status !== 'connected'}
          >
            Share Screen
          </Button>
        )
      }
    />
  )
}
