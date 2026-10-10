import { openPdf, pageOf, pdfOutline, renderPage, tocLabelForPage } from '@/lib/books/pdf'

const mockGetDocument = jest.fn()
const workerOptions = { workerSrc: '' }

jest.mock(
  'pdfjs-dist/legacy/build/pdf.mjs',
  () => ({
    GlobalWorkerOptions: workerOptions,
    getDocument: (...a: unknown[]) => mockGetDocument(...a)
  }),
  { virtual: true }
)

describe('pageOf', () => {
  it('reads #page=N links only', () => {
    expect(pageOf('#page=4')).toBe(4)
    expect(pageOf('#page=0')).toBeNull()
    expect(pageOf('#page=x')).toBeNull()
    expect(pageOf('res/a.xhtml')).toBeNull()
  })
})

describe('openPdf', () => {
  it('opens a blob from its bytes and a URL with credentials', async () => {
    const destroy = jest.fn(async () => {})
    mockGetDocument.mockReturnValue({ promise: Promise.resolve({ numPages: 2 }), destroy })
    const blob = new Blob(['%PDF'])
    blob.arrayBuffer = () => Promise.resolve(new TextEncoder().encode('%PDF').buffer)
    const opened = await openPdf(blob)
    expect(opened.pdf).toEqual({ numPages: 2 })
    expect(mockGetDocument.mock.calls[0]![0].data).toBeInstanceOf(Uint8Array)
    await opened.destroy()
    expect(destroy).toHaveBeenCalled()
    expect(workerOptions.workerSrc).toContain('pdf.worker')

    await openPdf('https://api.test/x.pdf')
    expect(mockGetDocument).toHaveBeenLastCalledWith({
      url: 'https://api.test/x.pdf',
      withCredentials: true
    })
  })
})

describe('pdfOutline', () => {
  const pdf = {
    getDestination: jest.fn(async (name: string) => (name === 'named' ? [{ num: 1 }] : null)),
    getPageIndex: jest.fn(async () => 4),
    getOutline: jest.fn()
  }

  it('resolves explicit, named and unresolvable destinations', async () => {
    pdf.getOutline.mockResolvedValue([
      {
        title: 'One',
        dest: [2],
        items: [{ title: 'Named', dest: 'named', items: [] }]
      },
      { title: 'Broken', dest: 'missing', items: [] },
      { title: 'None', dest: null, items: [] }
    ])
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- a minimal document
    const items = await pdfOutline(pdf as never)
    expect(items[0]).toMatchObject({ label: 'One', href: '#page=3' })
    expect(items[0]!.subitems![0]).toMatchObject({ label: 'Named', href: '#page=5' })
    expect(items[1]!.href).toBeUndefined()
    expect(items[2]!.href).toBeUndefined()
  })

  it('is empty without an outline or when reading it fails', async () => {
    pdf.getOutline.mockResolvedValue(null)
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- a minimal document
    await expect(pdfOutline(pdf as never)).resolves.toEqual([])
    pdf.getOutline.mockRejectedValue(new Error('x'))
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- a minimal document
    await expect(pdfOutline(pdf as never)).resolves.toEqual([])
  })

  it('tolerates a destination lookup that throws', async () => {
    pdf.getDestination.mockRejectedValueOnce(new Error('x'))
    pdf.getOutline.mockResolvedValue([{ title: 'Bad', dest: 'named', items: [] }])
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- a minimal document
    const items = await pdfOutline(pdf as never)
    expect(items[0]!.href).toBeUndefined()
  })
})

describe('tocLabelForPage', () => {
  const toc = [
    { label: 'A', href: '#page=1', subitems: [{ label: 'A.1', href: '#page=3' }] },
    { label: 'Group' },
    { label: 'B', href: '#page=6' }
  ]
  it('picks the last entry at or before the page', () => {
    expect(tocLabelForPage(toc, 1)).toBe('A')
    expect(tocLabelForPage(toc, 4)).toBe('A.1')
    expect(tocLabelForPage(toc, 9)).toBe('B')
    expect(tocLabelForPage([], 1)).toBeUndefined()
  })
})

describe('renderPage', () => {
  function setup(render: () => { promise: Promise<void>; cancel: () => void }) {
    const viewport = (scale: number) => ({ width: 200 * scale, height: 400 * scale })
    const page = {
      getViewport: ({ scale }: { scale: number }) => viewport(scale),
      render
    }
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- a minimal document
    const pdf = { getPage: async () => page } as never
    return { pdf, canvas: document.createElement('canvas') }
  }

  it('fits the page to the area at device resolution', async () => {
    const { pdf, canvas } = setup(() => ({ promise: Promise.resolve(), cancel: jest.fn() }))
    await expect(renderPage(pdf, 1, canvas, 100, 100)).resolves.toBe(true)
    expect(canvas.style.width).toBe('50px')
    expect(canvas.style.height).toBe('100px')
  })

  it('reports a cancelled render and rethrows other failures', async () => {
    const cancelled = Object.assign(new Error('c'), { name: 'RenderingCancelledException' })
    let a = setup(() => ({ promise: Promise.reject(cancelled), cancel: jest.fn() }))
    await expect(renderPage(a.pdf, 1, a.canvas, 100, 100)).resolves.toBe(false)
    a = setup(() => ({ promise: Promise.reject(new Error('boom')), cancel: jest.fn() }))
    await expect(renderPage(a.pdf, 1, a.canvas, 100, 100)).rejects.toThrow('boom')
  })

  it('cancels the render on abort and skips an already aborted one', async () => {
    const cancel = jest.fn()
    let reject: (e: Error) => void = () => {}
    const a = setup(() => ({
      promise: new Promise<void>((_, r) => (reject = r)),
      cancel
    }))
    const controller = new AbortController()
    const done = renderPage(a.pdf, 1, a.canvas, 100, 100, controller.signal)
    await Promise.resolve()
    await Promise.resolve()
    controller.abort()
    expect(cancel).toHaveBeenCalled()
    reject(Object.assign(new Error('c'), { name: 'RenderingCancelledException' }))
    await expect(done).resolves.toBe(false)
    await expect(renderPage(a.pdf, 1, a.canvas, 100, 100, controller.signal)).resolves.toBe(false)
  })
})
