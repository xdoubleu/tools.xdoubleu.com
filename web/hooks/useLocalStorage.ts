import { useState, useEffect } from 'react'

/**
 * localStorage-backed state, read synchronously on first render so stored
 * layout choices don't flash. Key convention: `<area>:<name>`.
 */
function readStored<T>(key: string, fallback: T): T {
  try {
    const item = localStorage.getItem(key)
    if (item === null) return fallback
    const parsed: T = JSON.parse(item)
    return parsed
  } catch {
    return fallback
  }
}

export function useLocalStorage<T>(key: string, initialValue: T): [T, (value: T) => void] {
  const [storedValue, setStoredValue] = useState<T>(() => readStored(key, initialValue))

  // A changed key re-reads; the initial read happened above.
  useEffect(() => {
    const item = readStored<T | null>(key, null)
    if (item !== null) setStoredValue(item)
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
