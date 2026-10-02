'use client'

import { useEffect } from 'react'
import { usePathname } from 'next/navigation'
import { registerServiceWorker, savePageForOffline } from '@/lib/offline/session'

export default function ServiceWorkerRegistrar() {
  const pathname = usePathname()

  useEffect(() => {
    registerServiceWorker()
  }, [])

  useEffect(() => {
    savePageForOffline(window.location.href)
  }, [pathname])

  return null
}
