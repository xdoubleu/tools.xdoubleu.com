const mockReadium = jest.fn()
const mockPdf = jest.fn()
jest.mock('@/components/books/reader/ReadiumBookReader', () => {
  mockReadium()
  return { __esModule: true, default: () => null }
})
jest.mock('@/components/books/reader/PdfBookReader', () => {
  mockPdf()
  return { __esModule: true, default: () => null }
})
jest.mock('@/lib/books/pdf', () => ({ pdfWorkerUrl: () => '/_next/static/media/pdf.worker.mjs' }))

import { warmReaderModules } from '@/lib/books/readerModules'

const fetchMock = jest.fn()
const realFetch = global.fetch

function setController(controller: object | null) {
  Object.defineProperty(navigator, 'serviceWorker', {
    value: { controller },
    configurable: true
  })
}

beforeEach(() => {
  jest.clearAllMocks()
  global.fetch = fetchMock
  fetchMock.mockResolvedValue({ ok: true })
})
afterEach(() => {
  global.fetch = realFetch
})

describe('warmReaderModules', () => {
  it('does nothing without a controlling worker', async () => {
    setController(null)
    await warmReaderModules()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('loads both readers and the worker, retrying after a failure, then stops', async () => {
    setController({})
    fetchMock.mockResolvedValueOnce({ ok: false, status: 404 })
    await warmReaderModules()
    expect(fetchMock).toHaveBeenCalledTimes(1)

    await warmReaderModules()
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(fetchMock).toHaveBeenLastCalledWith('/_next/static/media/pdf.worker.mjs')
    expect(mockReadium).toHaveBeenCalled()
    expect(mockPdf).toHaveBeenCalled()

    await warmReaderModules()
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })
})
