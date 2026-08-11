'use client'

import { useCallback, useEffect, useRef, useState } from 'react'

/**
 * Wraps the Screen Wake Lock API. The browser silently releases the lock
 * when the tab is hidden, so `wantedRef` tracks user intent separately from
 * `isActive` and re-requests the lock on `visibilitychange` if still wanted.
 */
export function useWakeLock() {
  const [isActive, setIsActive] = useState(false)
  const sentinelRef = useRef<WakeLockSentinel | null>(null)
  const wantedRef = useRef(false)
  const isSupported = typeof navigator !== 'undefined' && 'wakeLock' in navigator

  const request = useCallback(async () => {
    if (!isSupported) return
    try {
      const sentinel = await navigator.wakeLock.request('screen')
      sentinelRef.current = sentinel
      setIsActive(true)
      sentinel.addEventListener('release', () => {
        sentinelRef.current = null
        setIsActive(false)
      })
    } catch {
      setIsActive(false)
    }
  }, [isSupported])

  const enable = useCallback(() => {
    wantedRef.current = true
    void request()
  }, [request])

  const disable = useCallback(async () => {
    wantedRef.current = false
    await sentinelRef.current?.release()
  }, [])

  const toggle = useCallback(() => {
    if (wantedRef.current) {
      void disable()
    } else {
      enable()
    }
  }, [enable, disable])

  useEffect(() => {
    if (!isSupported) return
    const handleVisibilityChange = () => {
      if (wantedRef.current && document.visibilityState === 'visible' && !sentinelRef.current) {
        void request()
      }
    }
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => document.removeEventListener('visibilitychange', handleVisibilityChange)
  }, [isSupported, request])

  useEffect(() => {
    return () => {
      wantedRef.current = false
      void sentinelRef.current?.release()
    }
  }, [])

  return { isActive, isSupported, toggle }
}
