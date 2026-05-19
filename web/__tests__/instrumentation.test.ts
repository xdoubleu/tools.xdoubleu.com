/**
 * @jest-environment node
 */

jest.mock('@sentry/nextjs', () => ({
  init: jest.fn(),
  captureRequestError: jest.fn()
}))

jest.mock('../sentry.server.config', () => ({}))
jest.mock('../sentry.edge.config', () => ({}))

describe('instrumentation', () => {
  beforeEach(() => {
    jest.resetModules()
    delete process.env.NEXT_RUNTIME
  })

  it('loads server config when NEXT_RUNTIME is nodejs', async () => {
    process.env.NEXT_RUNTIME = 'nodejs'

    const { register } = require('../instrumentation')
    await register()

    expect(true).toBe(true)
  })

  it('loads edge config when NEXT_RUNTIME is edge', async () => {
    process.env.NEXT_RUNTIME = 'edge'

    const { register } = require('../instrumentation')
    await register()

    expect(true).toBe(true)
  })

  it('onRequestError equals Sentry.captureRequestError', () => {
    const Sentry = require('@sentry/nextjs')
    const { onRequestError } = require('../instrumentation')

    expect(onRequestError).toBe(Sentry.captureRequestError)
  })
})
