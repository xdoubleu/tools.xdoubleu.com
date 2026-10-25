import { useCallback, useState } from 'react'

/**
 * Runs a mutation and keeps `failure` in `error` for an `Alert` when it
 * rejects. `run` never rejects; it resolves to whether the action succeeded.
 */
export function useAsyncAction() {
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')

  const run = useCallback(
    async (action: () => Promise<unknown>, failure: string): Promise<boolean> => {
      setPending(true)
      setError('')
      try {
        await action()
        return true
      } catch {
        setError(failure)
        return false
      } finally {
        setPending(false)
      }
    },
    []
  )

  return { run, pending, error }
}
