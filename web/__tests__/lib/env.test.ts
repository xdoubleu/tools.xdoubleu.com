/**
 * @jest-environment jsdom
 */

import { getRelease, getApiUrl, getSentryDsn, getKoboGatewayRelease } from '@/lib/env'

describe('getRelease', () => {
  const originalEnv = process.env
  const originalWindow = global.window

  beforeEach(() => {
    jest.resetModules()
    process.env = { ...originalEnv }
    // Reset window.__ENV__ for each test
    if (typeof window !== 'undefined') {
      window.__ENV__ = { API_URL: '', SENTRY_DSN_WEB: '', RELEASE: '', KOBO_GATEWAY_RELEASE: '' }
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
      KOBO_GATEWAY_RELEASE: ''
    }

    const release = getRelease()
    expect(release).toBe('abc123def456')
  })

  it('returns empty string when window.__ENV__.RELEASE is not set', () => {
    window.__ENV__ = { API_URL: '', SENTRY_DSN_WEB: '', RELEASE: '', KOBO_GATEWAY_RELEASE: '' }

    const release = getRelease()
    expect(release).toBe('')
  })

  it('returns empty string when window.__ENV__ is not defined', () => {
    window.__ENV__ = { API_URL: '', SENTRY_DSN_WEB: '', RELEASE: '', KOBO_GATEWAY_RELEASE: '' }

    const release = getRelease()
    expect(release).toBe('')
  })

  it('prefers window.__ENV__.RELEASE over process.env.RELEASE in browser', () => {
    window.__ENV__ = {
      API_URL: '',
      SENTRY_DSN_WEB: '',
      RELEASE: 'browser-release',
      KOBO_GATEWAY_RELEASE: ''
    }
    process.env.RELEASE = 'process-release'

    const release = getRelease()
    expect(release).toBe('browser-release')
  })

  it('returns empty string when neither is set', () => {
    window.__ENV__ = { API_URL: '', SENTRY_DSN_WEB: '', RELEASE: '', KOBO_GATEWAY_RELEASE: '' }
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
      KOBO_GATEWAY_RELEASE: 'def789abc012'
    }

    expect(getKoboGatewayRelease()).toBe('def789abc012')
  })

  it('defaults to dev when window.__ENV__.KOBO_GATEWAY_RELEASE is not set', () => {
    window.__ENV__ = { API_URL: '', SENTRY_DSN_WEB: '', RELEASE: '', KOBO_GATEWAY_RELEASE: '' }

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
      KOBO_GATEWAY_RELEASE: ''
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
      KOBO_GATEWAY_RELEASE: ''
    }

    const sentryDsn = getSentryDsn()
    expect(sentryDsn).toBe('https://sentry.example.com/dsn')
  })
})
