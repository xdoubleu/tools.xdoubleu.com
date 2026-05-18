import {
  getPolicyStorageKey,
  isPolicyBannerDismissed,
  dismissPolicyBanner,
  clearPolicyBannerState,
} from '@/lib/todos/policiesBanner'

// Use a simple in-memory localStorage mock that works in jsdom.
// jsdom provides localStorage by default; we just clear it between tests.

beforeEach(() => {
  localStorage.clear()
})

afterEach(() => {
  clearPolicyBannerState()
  jest.restoreAllMocks()
})

describe('getPolicyStorageKey', () => {
  it('returns key prefixed with "policies:"', () => {
    expect(getPolicyStorageKey(['a', 'b'])).toMatch(/^policies:/)
  })

  it('sorts IDs before joining so key is order-independent', () => {
    expect(getPolicyStorageKey(['b', 'a'])).toBe(getPolicyStorageKey(['a', 'b']))
  })

  it('produces different keys for different policy ID sets', () => {
    const key1 = getPolicyStorageKey(['policy-1'])
    const key2 = getPolicyStorageKey(['policy-2'])
    expect(key1).not.toBe(key2)
  })

  it('handles a single policy ID', () => {
    expect(getPolicyStorageKey(['only'])).toBe('policies:only')
  })
})

describe('isPolicyBannerDismissed', () => {
  it('returns false when nothing is stored', () => {
    expect(isPolicyBannerDismissed(['p1', 'p2'], 24)).toBe(false)
  })

  it('returns true when dismissed just now (within reappearAfterHours)', () => {
    dismissPolicyBanner(['p1'])
    expect(isPolicyBannerDismissed(['p1'], 24)).toBe(true)
  })

  it('returns false when dismissed long ago (past reappear threshold)', () => {
    const key = getPolicyStorageKey(['p1'])
    // Store a timestamp 25 hours in the past
    const pastTimestamp = Date.now() - 25 * 3600 * 1000
    localStorage.setItem(key, String(pastTimestamp))
    expect(isPolicyBannerDismissed(['p1'], 24)).toBe(false)
  })

  it('returns true when dismissed just before threshold', () => {
    const key = getPolicyStorageKey(['p1'])
    // Store a timestamp 23 hours ago — still within 24-hour window
    const recentTimestamp = Date.now() - 23 * 3600 * 1000
    localStorage.setItem(key, String(recentTimestamp))
    expect(isPolicyBannerDismissed(['p1'], 24)).toBe(true)
  })

  it('returns false for empty policyIds array', () => {
    expect(isPolicyBannerDismissed([], 24)).toBe(false)
  })

  it('uses Date.now() for comparison — mock advances time past threshold', () => {
    dismissPolicyBanner(['p-time'])
    // Advance Date.now by 25 hours
    const original = Date.now
    jest.spyOn(Date, 'now').mockReturnValue(original() + 25 * 3600 * 1000)
    expect(isPolicyBannerDismissed(['p-time'], 24)).toBe(false)
  })

  it('different policy sets are checked independently', () => {
    dismissPolicyBanner(['set-a'])
    expect(isPolicyBannerDismissed(['set-a'], 24)).toBe(true)
    expect(isPolicyBannerDismissed(['set-b'], 24)).toBe(false)
  })

  it('returns false when stored value is not a valid number', () => {
    const key = getPolicyStorageKey(['bad'])
    localStorage.setItem(key, 'not-a-number')
    expect(isPolicyBannerDismissed(['bad'], 24)).toBe(false)
  })
})

describe('dismissPolicyBanner', () => {
  it('stores a timestamp in localStorage', () => {
    dismissPolicyBanner(['p1'])
    const key = getPolicyStorageKey(['p1'])
    const stored = localStorage.getItem(key)
    expect(stored).not.toBeNull()
    expect(Number(stored)).toBeGreaterThan(0)
  })

  it('does nothing for empty policyIds array', () => {
    dismissPolicyBanner([])
    expect(localStorage.length).toBe(0)
  })
})

describe('clearPolicyBannerState', () => {
  it('removes all policy banner keys from localStorage', () => {
    dismissPolicyBanner(['p1'])
    dismissPolicyBanner(['p2', 'p3'])
    clearPolicyBannerState()
    expect(isPolicyBannerDismissed(['p1'], 24)).toBe(false)
    expect(isPolicyBannerDismissed(['p2', 'p3'], 24)).toBe(false)
  })

  it('does not remove unrelated localStorage keys', () => {
    localStorage.setItem('other-key', 'value')
    dismissPolicyBanner(['p1'])
    clearPolicyBannerState()
    expect(localStorage.getItem('other-key')).toBe('value')
  })
})
