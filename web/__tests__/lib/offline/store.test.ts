/** @jest-environment node */
import 'fake-indexeddb/auto'
import {
  clearStore,
  getOwner,
  loadEntry,
  pruneEntries,
  saveEntry,
  setOwner
} from '@/lib/offline/store'

describe('offline store', () => {
  beforeEach(async () => {
    await clearStore()
  })

  it('round-trips an entry with its save time', async () => {
    jest.spyOn(Date, 'now').mockReturnValue(1000)
    await saveEntry('k', { n: 1n, bytes: new Uint8Array([1]) })
    jest.restoreAllMocks()

    expect(await loadEntry('k')).toEqual({
      data: { n: 1n, bytes: new Uint8Array([1]) },
      savedAt: 1000
    })
    expect(await loadEntry('missing')).toBeUndefined()
  })

  it('stores the owner and clears it with the entries', async () => {
    await setOwner('u1')
    await saveEntry('k', 1)
    expect(await getOwner()).toBe('u1')

    await clearStore()

    expect(await getOwner()).toBeUndefined()
    expect(await loadEntry('k')).toBeUndefined()
  })

  it('prunes only entries saved before the cutoff', async () => {
    const now = jest.spyOn(Date, 'now')
    now.mockReturnValue(100)
    await saveEntry('old', 1)
    now.mockReturnValue(300)
    await saveEntry('new', 2)
    now.mockRestore()

    await pruneEntries(200)

    expect(await loadEntry('old')).toBeUndefined()
    expect(await loadEntry('new')).toEqual({ data: 2, savedAt: 300 })
  })
})

describe('offline store write failures', () => {
  it('swallows values IndexedDB cannot store', async () => {
    await expect(saveEntry('fn', () => {})).resolves.toBeUndefined()
    expect(await loadEntry('fn')).toBeUndefined()
  })
})

describe('offline store without IndexedDB', () => {
  it('resolves to undefined and no-ops', async () => {
    const original = globalThis.indexedDB
    // @ts-expect-error -- simulate a browser without IndexedDB
    delete globalThis.indexedDB
    try {
      await saveEntry('k', 1)
      expect(await loadEntry('k')).toBeUndefined()
      expect(await getOwner()).toBeUndefined()
    } finally {
      globalThis.indexedDB = original
    }
  })
})
