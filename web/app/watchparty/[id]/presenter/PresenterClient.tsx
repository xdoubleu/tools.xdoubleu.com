'use client'

import { useEffect, useRef, useState } from 'react'
import Link from 'next/link'
import { attachDraggable } from '@/lib/watchparty/roomUtils'
import { STATUS_COLOR, STATUS_LABEL } from '@/lib/watchparty/types'
import { useWatchPartyRTC, type ScreenControls } from '@/hooks/useWatchPartyRTC'

export default function PresenterClient({ id }: { id: string }) {
  const mainVideoRef = useRef<HTMLVideoElement>(null)
  const selfCamRef = useRef<HTMLVideoElement>(null)
  const remoteCamRef = useRef<HTMLVideoElement>(null)
  const screenControlsRef = useRef<ScreenControls>({
    start: () => Promise.resolve(),
    stop: () => {}
  })

  const [sharing, setSharing] = useState(false)

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
    role: 'presenter',
    mainVideoRef,
    selfCamRef,
    remoteCamRef,
    onSharingChange: setSharing,
    screenControls: screenControlsRef
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
        <h1 className="text-xl font-bold">Watch Party (Presenter)</h1>
        <div className="flex items-center gap-2 ml-auto">
          <span className={`w-2.5 h-2.5 rounded-full ${STATUS_COLOR[status]}`} />
          <span className="text-sm text-muted">{STATUS_LABEL[status]}</span>
        </div>
      </div>

      {error && <p className="px-4 py-2 text-sm text-danger bg-danger/10">{error}</p>}

      <div className="relative flex-1 bg-black overflow-hidden">
        <video
          ref={mainVideoRef}
          autoPlay
          playsInline
          muted
          className="w-full h-full object-contain"
        />

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
        {!sharing ? (
          <button
            onClick={() => void screenControlsRef.current.start()}
            disabled={status !== 'connected'}
            className="rounded-xl bg-accent px-4 py-2 text-sm font-medium text-white hover:bg-accent-hover disabled:opacity-50"
          >
            Share Screen
          </button>
        ) : (
          <button
            onClick={() => screenControlsRef.current.stop()}
            className="rounded-xl bg-danger px-4 py-2 text-sm font-medium text-white hover:opacity-90"
          >
            Stop Sharing
          </button>
        )}

        <button
          onClick={toggleMic}
          className={`px-4 py-1.5 rounded text-sm font-medium border ${
            micEnabled ? 'border-border hover:bg-accent' : 'bg-warn text-white border-warn'
          }`}
        >
          {micEnabled ? 'Mute Mic' : 'Unmute Mic'}
        </button>

        <button
          onClick={toggleCam}
          className={`px-4 py-1.5 rounded text-sm font-medium border ${
            camEnabled ? 'border-border hover:bg-accent' : 'bg-warn text-white border-warn'
          }`}
        >
          {camEnabled ? 'Disable Cam' : 'Enable Cam'}
        </button>

        <button
          onClick={toggleSelfCam}
          className={`px-4 py-1.5 rounded text-sm font-medium border ${
            selfCamVisible ? 'border-border hover:bg-accent' : 'border-border text-muted'
          }`}
        >
          {selfCamVisible ? 'Hide Self' : 'Show Self'}
        </button>

        <span className="ml-auto text-xs text-muted font-mono">Room: {id}</span>
      </div>
    </div>
  )
}
