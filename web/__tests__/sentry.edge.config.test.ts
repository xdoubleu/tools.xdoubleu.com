/**
 * @jest-environment node
 */

export {}

const mockInit = jest.fn()

jest.mock('@sentry/nextjs', () => ({
  init: mockInit
}))

describe('sentry.edge.config', () => {
  beforeEach(() => {
    jest.resetModules()
    mockInit.mockClear()
  })

  it('initializes sentry with config', () => {
    process.env.SENTRY_DSN = 'https://example@sentry.io/123'
    process.env.NEXT_PUBLIC_RELEASE = 'v1.0.0'

    require('../sentry.edge.config')

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

    require('../sentry.edge.config')

    expect(mockInit).toHaveBeenCalledWith(
      expect.objectContaining({
        release: 'dev'
      })
    )
  })
})
