import { createFoliateView } from '@/lib/books/foliate'

describe('createFoliateView', () => {
  it('loads foliate-js from its static copy and creates its view element', async () => {
    const load = jest.fn().mockResolvedValue({})
    const view = await createFoliateView(load)
    expect(load).toHaveBeenCalledWith('/foliate-js/view.js')
    expect(view.tagName).toBe('FOLIATE-VIEW')
  })

  it('imports the static copy by default, which only the browser can reach', async () => {
    await expect(createFoliateView()).rejects.toThrow(/foliate-js\/view/)
  })
})

describe('warmReaderModules', () => {
  const fetchMock = jest.fn<Promise<unknown>, [string]>()
  beforeEach(() => {
    fetchMock.mockReset().mockResolvedValue({ ok: true })
    Object.defineProperty(globalThis, 'fetch', { value: fetchMock, configurable: true })
  })

  function setController(controller: unknown) {
    Object.defineProperty(navigator, 'serviceWorker', {
      value: { controller },
      configurable: true
    })
  }

  // Each test gets its own once-per-session flag.
  function freshWarm(): () => Promise<void> {
    let warm!: () => Promise<void>
    jest.isolateModules(() => {
      warm =
        jest.requireActual<typeof import('@/lib/books/foliate')>(
          '@/lib/books/foliate'
        ).warmReaderModules
    })
    return warm
  }

  it('fetches the EPUB and PDF modules once, through the service worker', async () => {
    setController({})
    const warm = freshWarm()
    await warm()
    await warm()
    const urls = fetchMock.mock.calls.map(([url]) => url)
    expect(urls).toEqual(
      expect.arrayContaining([
        '/foliate-js/view.js',
        '/foliate-js/epub.js',
        '/foliate-js/pdf.js',
        '/foliate-js/vendor/pdfjs/pdf.mjs',
        '/foliate-js/vendor/pdfjs/pdf.worker.mjs'
      ])
    )
    expect(new Set(urls).size).toBe(urls.length)
  })

  it.each([
    ['a failed fetch', () => fetchMock.mockRejectedValueOnce(new Error('offline'))],
    ['an error response', () => fetchMock.mockResolvedValueOnce({ ok: false })]
  ])('keeps going past %s, and retries on the next call', async (_, fail) => {
    setController({})
    fail()
    const warm = freshWarm()
    await warm()
    const perRun = fetchMock.mock.calls.length
    expect(perRun).toBeGreaterThan(1)

    await warm()
    expect(fetchMock).toHaveBeenCalledTimes(perRun * 2)
    await warm()
    expect(fetchMock).toHaveBeenCalledTimes(perRun * 2)
  })

  it('does nothing without a controlling service worker', async () => {
    setController(null)
    await freshWarm()()
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
