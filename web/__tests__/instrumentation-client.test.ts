export {}

const mockSentryInit = jest.fn()
const mockPostHogInit = jest.fn()

jest.mock('@sentry/nextjs', () => ({
  init: mockSentryInit,
  captureRouterTransitionStart: jest.fn()
}))

jest.mock('posthog-js', () => ({
  __esModule: true,
  default: { init: mockPostHogInit }
}))

jest.mock('@/lib/env', () => ({
  getSentryDsn: jest.fn(() => 'https://example@sentry.io/123'),
  getRelease: jest.fn(() => 'v1.0.0'),
  getPostHogKey: jest.fn(),
  getPostHogHost: jest.fn(() => 'https://eu.i.posthog.com')
}))

describe('instrumentation-client', () => {
  beforeEach(() => {
    jest.resetModules()
    mockSentryInit.mockClear()
    mockPostHogInit.mockClear()
  })

  it('always initializes Sentry', () => {
    require('../instrumentation-client')

    expect(mockSentryInit).toHaveBeenCalledWith(
      expect.objectContaining({ dsn: 'https://example@sentry.io/123', release: 'v1.0.0' })
    )
  })

  it('initializes PostHog when a key is configured', () => {
    jest.mocked(require('@/lib/env').getPostHogKey).mockReturnValue('phc_abc123')

    require('../instrumentation-client')

    expect(mockPostHogInit).toHaveBeenCalledWith('phc_abc123', {
      api_host: 'https://eu.i.posthog.com',
      person_profiles: 'identified_only',
      capture_pageview: true,
      capture_pageleave: true
    })
  })

  it('skips PostHog initialization when no key is configured', () => {
    jest.mocked(require('@/lib/env').getPostHogKey).mockReturnValue('')

    require('../instrumentation-client')

    expect(mockPostHogInit).not.toHaveBeenCalled()
  })
})
