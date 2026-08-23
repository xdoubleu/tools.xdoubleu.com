import { logger } from '@/lib/logger'

const mockFetch = jest.fn()

describe('logger (client)', () => {
  beforeEach(() => {
    jest.useFakeTimers()
    mockFetch.mockReset()
    mockFetch.mockResolvedValue({ ok: true })
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
    global.fetch = mockFetch as unknown as typeof fetch
  })

  afterEach(() => {
    jest.useRealTimers()
  })

  it('posts to the local /logs route instead of the api directly', async () => {
    const infoSpy = jest.spyOn(console, 'info').mockImplementation(() => {})

    logger.info('from the browser')
    jest.advanceTimersByTime(2000)
    await Promise.resolve()
    await Promise.resolve()

    expect(mockFetch).toHaveBeenCalledWith('/logs', expect.objectContaining({ method: 'POST' }))
    infoSpy.mockRestore()
  })
})
