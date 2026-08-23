/**
 * @jest-environment node
 */
import { logger } from '@/lib/logger'

const mockFetch = jest.fn()

describe('logger (server)', () => {
  const originalSecret = process.env.OBSERVABILITY_INGEST_SECRET
  const originalApiUrl = process.env.API_URL

  beforeEach(() => {
    jest.useFakeTimers()
    mockFetch.mockReset()
    mockFetch.mockResolvedValue({ ok: true })
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
    global.fetch = mockFetch as unknown as typeof fetch
    process.env.OBSERVABILITY_INGEST_SECRET = 'test-secret'
    process.env.API_URL = 'https://api.example.com'
  })

  afterEach(() => {
    jest.useRealTimers()
    process.env.OBSERVABILITY_INGEST_SECRET = originalSecret
    process.env.API_URL = originalApiUrl
  })

  it('logs locally and batches a debounced POST to the api ingest endpoint', async () => {
    const infoSpy = jest.spyOn(console, 'info').mockImplementation(() => {})

    logger.info('hello', { foo: 'bar' })
    expect(infoSpy).toHaveBeenCalledWith('hello', { foo: 'bar' })
    expect(mockFetch).not.toHaveBeenCalled()

    jest.advanceTimersByTime(2000)
    await Promise.resolve()
    await Promise.resolve()

    expect(mockFetch).toHaveBeenCalledWith(
      'https://api.example.com/api/observability/logs',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({
          'X-Observability-Ingest-Secret': 'test-secret'
        })
      })
    )
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
    const calls = mockFetch.mock.calls as unknown as [string, RequestInit][]
    const [, init] = calls[0]
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
    const body = JSON.parse(String(init.body)) as { entries: { level: string; message: string }[] }
    expect(body.entries).toHaveLength(1)
    expect(body.entries[0]).toMatchObject({ level: 'info', message: 'hello' })

    infoSpy.mockRestore()
  })

  it('does not send when no ingest secret is configured', async () => {
    process.env.OBSERVABILITY_INGEST_SECRET = ''
    const errorSpy = jest.spyOn(console, 'error').mockImplementation(() => {})

    logger.error('failed')
    jest.advanceTimersByTime(2000)
    await Promise.resolve()

    expect(mockFetch).not.toHaveBeenCalled()
    errorSpy.mockRestore()
  })

  it('swallows a failed upstream fetch', async () => {
    mockFetch.mockRejectedValue(new Error('network down'))
    const warnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {})

    logger.warn('uh oh')
    jest.advanceTimersByTime(2000)
    await Promise.resolve()
    await Promise.resolve()

    expect(mockFetch).toHaveBeenCalled()
    warnSpy.mockRestore()
  })

  it('flushes immediately once the batch reaches its max size', () => {
    const debugSpy = jest.spyOn(console, 'debug').mockImplementation(() => {})

    for (let i = 0; i < 25; i++) {
      logger.debug(`entry ${i}`)
    }

    expect(mockFetch).toHaveBeenCalledTimes(1)
    debugSpy.mockRestore()
  })
})
