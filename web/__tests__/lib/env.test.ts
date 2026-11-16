import {
  getApiUrl,
  getKoboGatewayRelease,
  getPostHogHost,
  getPostHogKey,
  getRelease,
  getSentryDsn
} from '@/lib/env'

const env = {
  API_URL: 'https://tools.example/api',
  SENTRY_DSN_WEB: 'https://key@sentry.example/1',
  RELEASE: 'abc123',
  KOBO_GATEWAY_RELEASE: 'def456',
  POSTHOG_KEY: 'phc_key',
  POSTHOG_HOST: 'https://eu.i.posthog.com'
}

describe('lib/env', () => {
  afterEach(() => {
    Reflect.deleteProperty(window, '__ENV__')
  })

  it('reads the config the api injects as window.__ENV__', () => {
    window.__ENV__ = env

    expect(getApiUrl()).toBe(env.API_URL)
    expect(getSentryDsn()).toBe(env.SENTRY_DSN_WEB)
    expect(getRelease()).toBe(env.RELEASE)
    expect(getKoboGatewayRelease()).toBe(env.KOBO_GATEWAY_RELEASE)
    expect(getPostHogKey()).toBe(env.POSTHOG_KEY)
    expect(getPostHogHost()).toBe(env.POSTHOG_HOST)
  })

  it('falls back to dev defaults without window.__ENV__', () => {
    expect(getApiUrl()).toBe('')
    expect(getSentryDsn()).toBe('')
    expect(getRelease()).toBe('dev')
    expect(getKoboGatewayRelease()).toBe('dev')
    expect(getPostHogKey()).toBe('')
    expect(getPostHogHost()).toBe('')
  })
})
