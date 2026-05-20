/**
 * @jest-environment jsdom
 */

import { getRelease, getApiUrl, getSentryDsn } from '@/lib/env'

describe('getRelease', () => {
  const originalEnv = process.env
  const originalWindow = global.window

  beforeEach(() => {
    jest.resetModules()
    process.env = { ...originalEnv }
    // Reset window.__ENV__ for each test
    if (typeof window !== 'undefined') {
      ;(window as any).__ENV__ = undefined
    }
  })

  afterEach(() => {
    process.env = originalEnv
    ;(global.window as any) = originalWindow
  })

  it('returns window.__ENV__.RELEASE when available', () => {
    ;(window as any).__ENV__ = {
      API_URL: '',
      SENTRY_DSN: '',
      RELEASE: 'abc123def456'
    }

    const release = getRelease()
    expect(release).toBe('abc123def456')
  })

  it('returns empty string when window.__ENV__.RELEASE is not set', () => {
    ;(window as any).__ENV__ = {
      API_URL: '',
      SENTRY_DSN: ''
    }

    const release = getRelease()
    expect(release).toBe('')
  })

  it('returns empty string when window.__ENV__ is not defined', () => {
    ;(window as any).__ENV__ = undefined

    const release = getRelease()
    expect(release).toBe('')
  })

  it('prefers window.__ENV__.RELEASE over process.env.RELEASE in browser', () => {
    ;(window as any).__ENV__ = {
      API_URL: '',
      SENTRY_DSN: '',
      RELEASE: 'browser-release'
    }
    process.env.RELEASE = 'process-release'

    const release = getRelease()
    expect(release).toBe('browser-release')
  })

  it('returns empty string when neither is set', () => {
    ;(window as any).__ENV__ = { API_URL: '', SENTRY_DSN: '' }
    delete process.env.RELEASE

    const release = getRelease()
    expect(release).toBe('')
  })
})

describe('getApiUrl', () => {
  beforeEach(() => {
    jest.resetModules()
  })

  it('returns window.__ENV__.API_URL when available', () => {
    ;(window as any).__ENV__ = {
      API_URL: 'https://api.example.com',
      SENTRY_DSN: '',
      RELEASE: ''
    }

    const apiUrl = getApiUrl()
    expect(apiUrl).toBe('https://api.example.com')
  })
})

describe('getSentryDsn', () => {
  beforeEach(() => {
    jest.resetModules()
  })

  it('returns window.__ENV__.SENTRY_DSN when available', () => {
    ;(window as any).__ENV__ = {
      API_URL: '',
      SENTRY_DSN: 'https://sentry.example.com/dsn',
      RELEASE: ''
    }

    const sentryDsn = getSentryDsn()
    expect(sentryDsn).toBe('https://sentry.example.com/dsn')
  })
})
