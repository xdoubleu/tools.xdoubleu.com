/**
 * @jest-environment node
 */

jest.mock('@sentry/nextjs', () => ({
  init: jest.fn()
}))

describe('sentry.server.config', () => {
  beforeEach(() => {
    jest.resetModules()
    delete (process.env as Record<string, string | undefined>).NODE_ENV
  })

  it('sets debug=false when NODE_ENV=production', () => {
    ;(process.env as Record<string, string>).NODE_ENV = 'production'

    const Sentry = require('@sentry/nextjs')
    const mockInit = Sentry.init as jest.Mock
    mockInit.mockClear()

    require('../sentry.server.config')

    expect(mockInit).toHaveBeenCalledWith(
      expect.objectContaining({
        debug: false
      })
    )
  })

  it('sets debug=true when NODE_ENV=development', () => {
    ;(process.env as Record<string, string>).NODE_ENV = 'development'

    const Sentry = require('@sentry/nextjs')
    const mockInit = Sentry.init as jest.Mock
    mockInit.mockClear()

    require('../sentry.server.config')

    expect(mockInit).toHaveBeenCalledWith(
      expect.objectContaining({
        debug: true
      })
    )
  })
})
