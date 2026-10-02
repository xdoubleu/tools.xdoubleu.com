import { loadEntry } from '@/lib/offline/store'

describe('offline store when IndexedDB fails to open', () => {
  it('resolves to undefined and retries the open next time', async () => {
    const open = jest.fn(() => {
      const req: { error: null; onerror?: () => void } = { error: null }
      setTimeout(() => req.onerror?.())
      return req
    })
    Object.defineProperty(globalThis, 'indexedDB', { value: { open }, configurable: true })

    expect(await loadEntry('k')).toBeUndefined()
    expect(await loadEntry('k')).toBeUndefined()
    expect(open).toHaveBeenCalledTimes(2)
  })
})
