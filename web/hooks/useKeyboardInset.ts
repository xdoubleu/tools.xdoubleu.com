'use client'

import { useEffect, useState } from 'react'

/**
 * Pixels of the layout viewport's bottom hidden behind the on-screen keyboard.
 * iOS (and Android Chrome by default) shrink only the visual viewport, so a
 * `bottom: 0` fixed element stays under the keyboard without this. 0 when
 * `visualViewport` is unsupported.
 */
export function useKeyboardInset() {
  const [inset, setInset] = useState(0)

  useEffect(() => {
    const viewport = window.visualViewport
    if (!viewport) return
    const update = () =>
      setInset(Math.max(0, Math.round(window.innerHeight - viewport.height - viewport.offsetTop)))
    viewport.addEventListener('resize', update)
    viewport.addEventListener('scroll', update)
    return () => {
      viewport.removeEventListener('resize', update)
      viewport.removeEventListener('scroll', update)
    }
  }, [])

  return inset
}
