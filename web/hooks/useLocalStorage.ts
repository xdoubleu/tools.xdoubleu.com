'use client'

import { useState, useEffect } from 'react'

/**
 * SSR-safe localStorage hook. On the server (static export build) `window`
 * is undefined, so the initial value is used for the first render and the
 * stored value is applied on mount via useEffect.
 *
 * Key convention: `<area>:<name>` (e.g. "backlog:library:columns").
 */
export function useLocalStorage<T>(key: string, initialValue: T): [T, (value: T) => void] {
  const [storedValue, setStoredValue] = useState<T>(initialValue)

  useEffect(() => {
    try {
      const item = localStorage.getItem(key)
      if (item !== null) {
        // JSON.parse returns `any`; TypeScript allows assigning any to T without
        // an explicit assertion — the caller owns both the key and the value type.
        setStoredValue(JSON.parse(item))
      }
    } catch {
      // Ignore parse errors or missing localStorage (e.g. SSR).
    }
  }, [key])

  const setValue = (value: T) => {
    setStoredValue(value)
    try {
      localStorage.setItem(key, JSON.stringify(value))
    } catch {
      // Ignore write errors (storage full, private browsing, etc.).
    }
  }

  return [storedValue, setValue]
}
