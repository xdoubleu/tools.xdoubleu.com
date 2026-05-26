'use client'

import { useEffect, useRef } from 'react'
import Link from 'next/link'
import { attachDraggable } from '@/lib/watchparty/roomUtils'
import { STATUS_COLOR, STATUS_LABEL } from '@/lib/watchparty/types'
import { useWatchPartyRTC } from '@/hooks/useWatchPartyRTC'

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
        <Link href="/watchparty" className="text-blue-600 hover:underline text-sm">
          &larr; Back
        </Link>
        <h1 className="text-xl font-bold">Watch Party</h1>
        <div className="flex items-center gap-2 ml-auto">
          <span className={`w-2.5 h-2.5 rounded-full ${STATUS_COLOR[status]}`} />
          <span className="text-sm text-muted-foreground">{STATUS_LABEL[status]}</span>
        </div>
      </div>

      {error && <p className="px-4 py-2 text-red-600 text-sm bg-red-50">{error}</p>}

      <div className="relative flex-1 bg-black overflow-hidden">
        <video ref={mainVideoRef} autoPlay playsInline className="w-full h-full object-contain" />

        <video
          ref={selfCamRef}
          autoPlay
          playsInline
          muted
          className="absolute bottom-4 right-4 rounded-lg object-cover border-2 border-white/40 cursor-grab bg-gray-900"
          style={{ width: 200, height: 200, display: 'block', transform: 'scaleX(-1)' }}
        />

        <video
          ref={remoteCamRef}
          autoPlay
          playsInline
          className="absolute bottom-4 left-4 rounded-lg object-cover border-2 border-white/40 cursor-grab bg-gray-900"
          style={{ width: 200, height: 200, display: 'none' }}
        />
      </div>

      <div className="flex flex-wrap items-center gap-2 px-4 py-3 border-t border-border bg-background">
        <button
          onClick={toggleMic}
          className={`px-4 py-1.5 rounded text-sm font-medium border ${
            micEnabled
              ? 'border-border hover:bg-accent'
              : 'bg-yellow-500 text-white border-yellow-500'
          }`}
        >
          {micEnabled ? 'Mute Mic' : 'Unmute Mic'}
        </button>

        <button
          onClick={toggleCam}
          className={`px-4 py-1.5 rounded text-sm font-medium border ${
            camEnabled
              ? 'border-border hover:bg-accent'
              : 'bg-yellow-500 text-white border-yellow-500'
          }`}
        >
          {camEnabled ? 'Disable Cam' : 'Enable Cam'}
        </button>

        <button
          onClick={toggleSelfCam}
          className={`px-4 py-1.5 rounded text-sm font-medium border ${
            selfCamVisible ? 'border-border hover:bg-accent' : 'border-border text-muted-foreground'
          }`}
        >
          {selfCamVisible ? 'Hide Self' : 'Show Self'}
        </button>

        <span className="ml-auto text-xs text-muted-foreground font-mono">Room: {id}</span>
      </div>
    </div>
  )
}
