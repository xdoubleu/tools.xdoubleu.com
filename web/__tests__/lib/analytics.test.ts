const mockCapture = jest.fn()
const mockPostHog = { __loaded: false }

jest.mock('posthog-js', () => ({
  __esModule: true,
  default: {
    get __loaded() {
      return mockPostHog.__loaded
    },
    capture: (...args: unknown[]) => mockCapture(...args)
  }
}))

import { track } from '@/lib/analytics'

describe('track', () => {
  beforeEach(() => {
    mockCapture.mockClear()
  })

  it('captures the event once PostHog is initialised', () => {
    mockPostHog.__loaded = true
    track('book_progress_updated', { source: 'progress_cell' })
    expect(mockCapture).toHaveBeenCalledWith('book_progress_updated', { source: 'progress_cell' })
  })

  it('is a no-op when PostHog is not initialised', () => {
    mockPostHog.__loaded = false
    track('book_progress_updated')
    expect(mockCapture).not.toHaveBeenCalled()
  })
})
