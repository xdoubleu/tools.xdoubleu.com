const STORAGE_PREFIX = 'policies:'

/**
 * Returns the localStorage key for a given set of policy IDs.
 * IDs are sorted so that the key is stable regardless of order.
 */
export function getPolicyStorageKey(policyIds: string[]): string {
  return STORAGE_PREFIX + [...policyIds].sort().join(',')
}

/**
 * Returns true if the policy banner has been dismissed recently enough
 * that it should remain hidden.
 *
 * A dismissal is "still active" when:
 *   Date.now() - storedTimestamp < reappearAfterHours * 3600 * 1000
 */
export function isPolicyBannerDismissed(
  policyIds: string[],
  reappearAfterHours: number
): boolean {
  if (policyIds.length === 0) return false
  try {
    const key = getPolicyStorageKey(policyIds)
    const stored = localStorage.getItem(key)
    if (stored === null) return false
    const timestamp = parseInt(stored, 10)
    if (isNaN(timestamp)) return false
    const ageMs = Date.now() - timestamp
    return ageMs < reappearAfterHours * 3600 * 1000
  } catch {
    // localStorage may be unavailable in some environments
    return false
  }
}

/**
 * Records the current timestamp for the given policy IDs,
 * marking the banner as dismissed.
 */
export function dismissPolicyBanner(policyIds: string[]): void {
  if (policyIds.length === 0) return
  try {
    const key = getPolicyStorageKey(policyIds)
    localStorage.setItem(key, String(Date.now()))
  } catch {
    // localStorage may be unavailable in some environments
  }
}

/**
 * Removes all policy banner dismissal entries from localStorage.
 * Intended for use in tests.
 */
export function clearPolicyBannerState(): void {
  try {
    const keysToRemove: string[] = []
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i)
      if (key !== null && key.startsWith(STORAGE_PREFIX)) {
        keysToRemove.push(key)
      }
    }
    keysToRemove.forEach((key) => localStorage.removeItem(key))
  } catch {
    // localStorage may be unavailable in some environments
  }
}
