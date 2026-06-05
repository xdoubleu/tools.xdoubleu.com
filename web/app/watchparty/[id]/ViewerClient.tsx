'use client'

import { useEffect, useRef } from 'react'
import Link from 'next/link'
import { attachDraggable } from '@/lib/watchparty/roomUtils'
import { STATUS_COLOR, STATUS_LABEL } from '@/lib/watchparty/types'
import { useWatchPartyRTC } from '@/hooks/useWatchPartyRTC'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'

export default function ViewerClient({ id }: { id: string }) {
  const mainVideoRef = useRef<HTMLVideoElement>(null)
  const selfCamRef = useRef<HTMLVideoElement>(null)
  const remoteCamRef = useRef<HTMLVideoElement>(null)

  const {
    status,
    micEnabled,
    camEnabled,
    selfCamVisible,
    error,
    toggleMic,
    toggleCam,
    toggleSelfCam
  } = useWatchPartyRTC({
    id,
    role: 'viewer',
    mainVideoRef,
    selfCamRef,
    remoteCamRef
  })

  useEffect(() => {
    if (!selfCamRef.current) return
    return attachDraggable(selfCamRef.current)
  }, [])

  useEffect(() => {
    if (!remoteCamRef.current) return
    return attachDraggable(remoteCamRef.current)
  }, [])

  return (
    <div className="flex flex-col h-[calc(100vh-4rem)]">
      <div className="flex items-center gap-4 px-4 py-3 border-b border-border">
        <Link href="/watchparty" className="text-sm text-accent hover:underline">
          &larr; Back
        </Link>
        <h1 className="text-xl font-bold">Watch Party</h1>
        <div className="flex items-center gap-2 ml-auto">
          <span className={`w-2.5 h-2.5 rounded-full ${STATUS_COLOR[status]}`} />
          <span className="text-sm text-muted">{STATUS_LABEL[status]}</span>
        </div>
      </div>

      {error && <p className="px-4 py-2 text-sm text-danger bg-danger/10">{error}</p>}

      <div className="relative flex-1 bg-black overflow-hidden">
        <video ref={mainVideoRef} autoPlay playsInline className="w-full h-full object-contain" />

        <video
          ref={selfCamRef}
          autoPlay
          playsInline
          muted
          className="absolute bottom-4 right-4 cursor-grab rounded-lg border-2 border-white/20 bg-black object-cover"
          style={{ width: 200, height: 200, display: 'block', transform: 'scaleX(-1)' }}
        />

        <video
          ref={remoteCamRef}
          autoPlay
          playsInline
          className="absolute bottom-4 left-4 cursor-grab rounded-lg border-2 border-white/20 bg-black object-cover"
          style={{ width: 200, height: 200, display: 'none' }}
        />
      </div>

      <div className="flex flex-wrap items-center gap-2 px-4 py-3 border-t border-border bg-surface">
        <Button
          variant="secondary"
          size="sm"
          onClick={toggleMic}
          className={cn(!micEnabled && 'border-warn bg-warn text-white hover:bg-warn/90')}
        >
          {micEnabled ? 'Mute Mic' : 'Unmute Mic'}
        </Button>

        <Button
          variant="secondary"
          size="sm"
          onClick={toggleCam}
          className={cn(!camEnabled && 'border-warn bg-warn text-white hover:bg-warn/90')}
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

        <span className="ml-auto text-xs text-muted font-mono">Room: {id}</span>
      </div>
    </div>
  )
}
