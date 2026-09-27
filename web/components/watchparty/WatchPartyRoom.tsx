'use client'

import { useEffect, type ReactNode, type RefObject } from 'react'
import { Alert } from '@/components/ui/alert'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'
import { attachDraggable } from '@/lib/watchparty/roomUtils'
import { STATUS_COLOR, STATUS_LABEL, type ConnectionStatus } from '@/lib/watchparty/types'

export interface WatchPartyRoomProps {
  id: string
  /** Last breadcrumb item, e.g. "Room" or "Presenter". */
  crumb: string
  status: ConnectionStatus
  error: string | null
  micEnabled: boolean
  camEnabled: boolean
  selfCamVisible: boolean
  toggleMic: () => void
  toggleCam: () => void
  toggleSelfCam: () => void
  mainVideoRef: RefObject<HTMLVideoElement | null>
  selfCamRef: RefObject<HTMLVideoElement | null>
  remoteCamRef: RefObject<HTMLVideoElement | null>
  /** Mutes the main video, e.g. the presenter's own screen share. */
  mainMuted?: boolean
  /** Extra controls placed before the mic/cam buttons. */
  controls?: ReactNode
}

const pipClass =
  'absolute bottom-4 aspect-square w-24 cursor-grab rounded-lg border-2 border-white/20 bg-black object-cover sm:w-48'
const offClass = 'border-warn bg-warn text-white hover:bg-warn/90'

/** Watch-party room: status bar, main video with draggable camera PiPs, and the control bar. */
export default function WatchPartyRoom({
  id,
  crumb,
  status,
  error,
  micEnabled,
  camEnabled,
  selfCamVisible,
  toggleMic,
  toggleCam,
  toggleSelfCam,
  mainVideoRef,
  selfCamRef,
  remoteCamRef,
  mainMuted,
  controls
}: WatchPartyRoomProps) {
  useEffect(() => {
    if (!selfCamRef.current) return
    return attachDraggable(selfCamRef.current)
  }, [selfCamRef])

  useEffect(() => {
    if (!remoteCamRef.current) return
    return attachDraggable(remoteCamRef.current)
  }, [remoteCamRef])

  return (
    <div className="flex flex-col">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b border-border pb-3">
        <Breadcrumb items={[{ label: 'Watch Party', href: '/watchparty' }, { label: crumb }]} />
        <div className="ml-auto flex items-center gap-2">
          <span className={cn('h-2.5 w-2.5 rounded-full', STATUS_COLOR[status])} />
          <span className="text-sm text-muted">{STATUS_LABEL[status]}</span>
        </div>
      </div>

      {error && (
        <Alert tone="danger" className="mt-3">
          {error}
        </Alert>
      )}

      <div className="relative mt-3 h-[60dvh] overflow-hidden rounded-xl bg-black sm:h-[70dvh]">
        <video
          ref={mainVideoRef}
          autoPlay
          playsInline
          muted={mainMuted}
          className="h-full w-full object-contain"
        />

        <video
          ref={selfCamRef}
          autoPlay
          playsInline
          muted
          aria-label="Your camera"
          className={cn(pipClass, 'right-4')}
          style={{ display: 'block', transform: 'scaleX(-1)' }}
        />

        <video
          ref={remoteCamRef}
          autoPlay
          playsInline
          aria-label="Other camera"
          className={cn(pipClass, 'left-4')}
          style={{ display: 'none' }}
        />
      </div>

      <div className="sticky bottom-0 flex flex-wrap items-center gap-2 border-t border-border bg-surface pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))]">
        {controls}

        <Button
          variant="secondary"
          size="sm"
          onClick={toggleMic}
          className={cn(!micEnabled && offClass)}
        >
          {micEnabled ? 'Mute Mic' : 'Unmute Mic'}
        </Button>

        <Button
          variant="secondary"
          size="sm"
          onClick={toggleCam}
          className={cn(!camEnabled && offClass)}
        >
          {camEnabled ? 'Disable Cam' : 'Enable Cam'}
        </Button>

        <Button
          variant="secondary"
          size="sm"
          onClick={toggleSelfCam}
          className={cn(!selfCamVisible && 'text-muted')}
        >
          {selfCamVisible ? 'Hide Self' : 'Show Self'}
        </Button>

        <span className="ml-auto font-mono text-xs text-muted">Room: {id}</span>
      </div>
    </div>
  )
}
