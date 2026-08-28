/**
 * @jest-environment node
 *
 * lib/env.ts's getApiUrl/getSentryDsn/getRelease/getKoboGatewayRelease all
 * branch on `typeof window`; env.test.ts (jsdom) only ever exercises the
 * browser branch. This file runs under Node so `window` is genuinely
 * undefined, covering the process.env fallback each one falls back to on
 * the server.
 */

import { getApiUrl, getSentryDsn, getRelease, getKoboGatewayRelease } from '@/lib/env'

const originalEnv = process.env

beforeEach(() => {
  process.env = { ...originalEnv }
})

afterAll(() => {
  process.env = originalEnv
})

it('getApiUrl reads process.env.API_URL on the server', () => {
  process.env.API_URL = 'https://api.example.com'
  expect(getApiUrl()).toBe('https://api.example.com')
})

it('getSentryDsn reads process.env.SENTRY_DSN_WEB on the server', () => {
  process.env.SENTRY_DSN_WEB = 'https://sentry.example.com/dsn'
  expect(getSentryDsn()).toBe('https://sentry.example.com/dsn')
})

it('getRelease reads process.env.RELEASE on the server, defaulting to dev', () => {
  process.env.RELEASE = 'server-release'
  expect(getRelease()).toBe('server-release')

  delete process.env.RELEASE
  expect(getRelease()).toBe('dev')
})

it('getKoboGatewayRelease reads process.env.KOBO_GATEWAY_RELEASE on the server, defaulting to dev', () => {
  process.env.KOBO_GATEWAY_RELEASE = 'server-kobo-release'
  expect(getKoboGatewayRelease()).toBe('server-kobo-release')

  delete process.env.KOBO_GATEWAY_RELEASE
  expect(getKoboGatewayRelease()).toBe('dev')
})
