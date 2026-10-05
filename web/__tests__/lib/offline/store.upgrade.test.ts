/** @jest-environment node */
import 'fake-indexeddb/auto'
import { listBookFiles, listQueued, loadEntry } from '@/lib/offline/store'

function openV1(): Promise<void> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open('tools-offline', 1)
    req.onupgradeneeded = () => {
      req.result.createObjectStore('swr')
      req.result.createObjectStore('meta')
    }
    req.onsuccess = () => {
      const db = req.result
      const tx = db.transaction('swr', 'readwrite')
      tx.objectStore('swr').put({ data: 'kept', savedAt: 1 }, 'k')
      tx.oncomplete = () => {
        db.close()
        resolve()
      }
    }
    req.onerror = () => reject(req.error ?? new Error('open failed'))
  })
}

describe('offline store upgrade', () => {
  it('adds the queue and book stores to a version 1 database and keeps its entries', async () => {
    await openV1()

    expect(await loadEntry('k')).toEqual({ data: 'kept', savedAt: 1 })
    expect(await listQueued()).toEqual([])
    expect(await listBookFiles()).toEqual([])
  })
})
