/**
 * @jest-environment jsdom
 */

import {
  getRelease,
  getApiUrl,
  getSentryDsn,
  getKoboGatewayRelease,
  getPostHogKey,
  getPostHogHost
} from '@/lib/env'

describe('getRelease', () => {
  const originalEnv = process.env
  const originalWindow = global.window

  beforeEach(() => {
    jest.resetModules()
    process.env = { ...originalEnv }
    if (typeof window !== 'undefined') {
      window.__ENV__ = {
        API_URL: '',
        SENTRY_DSN_WEB: '',
        RELEASE: '',
        KOBO_GATEWAY_RELEASE: '',
        POSTHOG_KEY: '',
        POSTHOG_HOST: ''
      }
    }
  })

  afterEach(() => {
    process.env = originalEnv
    Object.assign(global, { window: originalWindow })
  })

  it('returns window.__ENV__.RELEASE when available', () => {
    window.__ENV__ = {
      API_URL: '',
      SENTRY_DSN_WEB: '',
      RELEASE: 'abc123def456',
      KOBO_GATEWAY_RELEASE: '',
      POSTHOG_KEY: '',
      POSTHOG_HOST: ''
    }

    const release = getRelease()
    expect(release).toBe('abc123def456')
  })

  it('returns empty string when window.__ENV__.RELEASE is not set', () => {
    window.__ENV__ = {
      API_URL: '',
      SENTRY_DSN_WEB: '',
      RELEASE: '',
      KOBO_GATEWAY_RELEASE: '',
      POSTHOG_KEY: '',
      POSTHOG_HOST: ''
    }

    const release = getRelease()
    expect(release).toBe('')
  })

  it('returns empty string when window.__ENV__ is not defined', () => {
    window.__ENV__ = {
      API_URL: '',
      SENTRY_DSN_WEB: '',
      RELEASE: '',
      KOBO_GATEWAY_RELEASE: '',
      POSTHOG_KEY: '',
      POSTHOG_HOST: ''
    }

    const release = getRelease()
    expect(release).toBe('')
  })

  it('prefers window.__ENV__.RELEASE over process.env.RELEASE in browser', () => {
    window.__ENV__ = {
      API_URL: '',
      SENTRY_DSN_WEB: '',
      RELEASE: 'browser-release',
      KOBO_GATEWAY_RELEASE: '',
      POSTHOG_KEY: '',
      POSTHOG_HOST: ''
    }
    process.env.RELEASE = 'process-release'

    const release = getRelease()
    expect(release).toBe('browser-release')
  })

  it('returns empty string when neither is set', () => {
    window.__ENV__ = {
      API_URL: '',
      SENTRY_DSN_WEB: '',
      RELEASE: '',
      KOBO_GATEWAY_RELEASE: '',
      POSTHOG_KEY: '',
      POSTHOG_HOST: ''
    }
    delete process.env.RELEASE

    const release = getRelease()
    expect(release).toBe('')
  })
})

describe('getKoboGatewayRelease', () => {
  const originalEnv = process.env
  const originalWindow = global.window

  beforeEach(() => {
    jest.resetModules()
    process.env = { ...originalEnv }
  })

  afterEach(() => {
    process.env = originalEnv
    Object.assign(global, { window: originalWindow })
  })

  it('returns window.__ENV__.KOBO_GATEWAY_RELEASE when available', () => {
    window.__ENV__ = {
      API_URL: '',
      SENTRY_DSN_WEB: '',
      RELEASE: '',
      KOBO_GATEWAY_RELEASE: 'def789abc012',
      POSTHOG_KEY: '',
      POSTHOG_HOST: ''
    }

    expect(getKoboGatewayRelease()).toBe('def789abc012')
  })

  it('defaults to dev when window.__ENV__.KOBO_GATEWAY_RELEASE is not set', () => {
    window.__ENV__ = {
      API_URL: '',
      SENTRY_DSN_WEB: '',
      RELEASE: '',
      KOBO_GATEWAY_RELEASE: '',
      POSTHOG_KEY: '',
      POSTHOG_HOST: ''
    }

    expect(getKoboGatewayRelease()).toBe('')
  })

  it('defaults to dev when window.__ENV__ itself is undefined', () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion
    window.__ENV__ = undefined as any

    expect(getKoboGatewayRelease()).toBe('dev')
  })
})

describe('getApiUrl', () => {
  beforeEach(() => {
    jest.resetModules()
  })

  it('returns window.__ENV__.API_URL when available', () => {
    window.__ENV__ = {
      API_URL: 'https://api.example.com',
      SENTRY_DSN_WEB: '',
      RELEASE: '',
      KOBO_GATEWAY_RELEASE: '',
      POSTHOG_KEY: '',
      POSTHOG_HOST: ''
    }

    const apiUrl = getApiUrl()
    expect(apiUrl).toBe('https://api.example.com')
  })
})

describe('getSentryDsn', () => {
  beforeEach(() => {
    jest.resetModules()
  })

  it('returns window.__ENV__.SENTRY_DSN_WEB when available', () => {
    window.__ENV__ = {
      API_URL: '',
      SENTRY_DSN_WEB: 'https://sentry.example.com/dsn',
      RELEASE: '',
      KOBO_GATEWAY_RELEASE: '',
      POSTHOG_KEY: '',
      POSTHOG_HOST: ''
    }

    const sentryDsn = getSentryDsn()
    expect(sentryDsn).toBe('https://sentry.example.com/dsn')
  })

  it('returns empty string when window.__ENV__ itself is undefined', () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion
    window.__ENV__ = undefined as any

    expect(getSentryDsn()).toBe('')
  })
})

describe('getPostHogKey', () => {
  beforeEach(() => {
    jest.resetModules()
  })

  it('returns window.__ENV__.POSTHOG_KEY when available', () => {
    window.__ENV__ = {
      API_URL: '',
      SENTRY_DSN_WEB: '',
      RELEASE: '',
      KOBO_GATEWAY_RELEASE: '',
      POSTHOG_KEY: 'phc_abc123',
      POSTHOG_HOST: ''
    }

    expect(getPostHogKey()).toBe('phc_abc123')
  })

  it('returns empty string when window.__ENV__ itself is undefined', () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion
    window.__ENV__ = undefined as any

    expect(getPostHogKey()).toBe('')
  })
})

describe('getPostHogHost', () => {
  beforeEach(() => {
    jest.resetModules()
  })

  it('returns window.__ENV__.POSTHOG_HOST when available', () => {
    window.__ENV__ = {
      API_URL: '',
      SENTRY_DSN_WEB: '',
      RELEASE: '',
      KOBO_GATEWAY_RELEASE: '',
      POSTHOG_KEY: '',
      POSTHOG_HOST: 'https://eu.i.posthog.com'
    }

    expect(getPostHogHost()).toBe('https://eu.i.posthog.com')
  })

  it('returns empty string when window.__ENV__ itself is undefined', () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion
    window.__ENV__ = undefined as any

    expect(getPostHogHost()).toBe('')
  })
})
