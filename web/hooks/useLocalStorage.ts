'use client'

import { useState, useEffect } from 'react'

/**
 * SSR-safe localStorage hook: renders initialValue first, applies the stored
 * value on mount. Key convention: `<area>:<name>`.
 */
export function useLocalStorage<T>(key: string, initialValue: T): [T, (value: T) => void] {
  const [storedValue, setStoredValue] = useState<T>(initialValue)

  useEffect(() => {
    try {
      const item = localStorage.getItem(key)
      if (item !== null) {
        setStoredValue(JSON.parse(item))
      }
    } catch {
      // Ignore parse errors or missing localStorage.
    }
  }, [key])

  const setValue = (value: T) => {
    setStoredValue(value)
    try {
      localStorage.setItem(key, JSON.stringify(value))
    } catch {
      // Ignore write errors (full, private browsing).
    }
  }

  return [storedValue, setValue]
}
