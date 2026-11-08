import { useCallback, useEffect, useRef, useState } from 'react'

/**
 * Copies text to the clipboard and reports it for `resetMs`, for a
 * "Copy" → "Copied!" button. Pass a `key` to `copy` when one hook serves
 * several buttons; `copiedKey` names the one that last succeeded.
 */
export function useCopyFeedback(resetMs = 2000) {
  const [copiedKey, setCopiedKey] = useState<string | null>(null)
  const [failed, setFailed] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  useEffect(() => () => clearTimeout(timer.current), [])

  const copy = useCallback(
    async (text: string, key = '') => {
      clearTimeout(timer.current)
      try {
        await navigator.clipboard.writeText(text)
      } catch {
        setCopiedKey(null)
        setFailed(true)
        return false
      }
      setFailed(false)
      setCopiedKey(key)
      timer.current = setTimeout(() => setCopiedKey(null), resetMs)
      return true
    },
    [resetMs]
  )

  return { copy, copied: copiedKey !== null, copiedKey, failed }
}
