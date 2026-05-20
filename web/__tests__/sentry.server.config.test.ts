/**
 * @jest-environment node
 */

jest.mock('@sentry/nextjs', () => ({
  init: jest.fn()
}))

describe('sentry.server.config', () => {
  beforeEach(() => {
    jest.resetModules()
  })

  it('initializes sentry with config', () => {
    process.env.SENTRY_DSN = 'https://example@sentry.io/123'
    process.env.NEXT_PUBLIC_RELEASE = 'v1.0.0'

    const Sentry = require('@sentry/nextjs')
    const mockInit = Sentry.init as jest.Mock
    mockInit.mockClear()

    require('../sentry.server.config')

    expect(mockInit).toHaveBeenCalledWith(
      expect.objectContaining({
        dsn: 'https://example@sentry.io/123',
        release: 'v1.0.0',
        tracesSampleRate: 1.0
      })
    )
  })

  it('uses dev as default release', () => {
    delete process.env.NEXT_PUBLIC_RELEASE

    const Sentry = require('@sentry/nextjs')
    const mockInit = Sentry.init as jest.Mock
    mockInit.mockClear()

    require('../sentry.server.config')

    expect(mockInit).toHaveBeenCalledWith(
      expect.objectContaining({
        release: 'dev'
      })
    )
  })
})
