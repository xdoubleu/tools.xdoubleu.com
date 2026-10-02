import {
  claimOfflineData,
  clearOfflineData,
  registerServiceWorker,
  savePageForOffline
} from '@/lib/offline/session'
import { clearStore, getOwner, pruneEntries, setOwner } from '@/lib/offline/store'
import { logger } from '@/lib/logger'

jest.mock('@/lib/offline/store', () => ({
  clearStore: jest.fn(async () => {}),
  getOwner: jest.fn(),
  pruneEntries: jest.fn(async () => {}),
  setOwner: jest.fn(async () => {})
}))
jest.mock('@/lib/logger', () => ({ logger: { warn: jest.fn() } }))

const postMessage = jest.fn()
const register = jest.fn()

function installServiceWorker() {
  Object.defineProperty(navigator, 'serviceWorker', {
    value: {
      register,
      controller: { postMessage },
      ready: Promise.resolve({ active: { postMessage } })
    },
    configurable: true
  })
}

function setNodeEnv(value: string) {
  Object.defineProperty(process.env, 'NODE_ENV', { value, configurable: true, writable: true })
}

describe('offline session', () => {
  const cachesDelete = jest.fn(async () => true)

  beforeEach(() => {
    jest.clearAllMocks()
    installServiceWorker()
    Object.defineProperty(globalThis, 'caches', {
      value: { delete: cachesDelete },
      configurable: true
    })
  })

  afterEach(() => {
    setNodeEnv('test')
  })

  it('registers the worker only in production', async () => {
    registerServiceWorker()
    expect(register).not.toHaveBeenCalled()

    setNodeEnv('production')
    register.mockRejectedValueOnce(new Error('denied'))
    registerServiceWorker()
    expect(register).toHaveBeenCalledWith('/sw.js')
    await Promise.resolve()
    expect(logger.warn).toHaveBeenCalledWith('service worker registration failed', {
      error: 'Error: denied'
    })
  })

  it('asks the active worker to save a page', async () => {
    savePageForOffline('https://tools.test/feeds')
    await Promise.resolve()
    await Promise.resolve()

    expect(postMessage).toHaveBeenCalledWith({ type: 'save-page', url: 'https://tools.test/feeds' })
  })

  it('clears the store, the page cache and the worker state', async () => {
    await clearOfflineData()

    expect(postMessage).toHaveBeenCalledWith({ type: 'clear' })
    expect(clearStore).toHaveBeenCalled()
    expect(cachesDelete).toHaveBeenCalledWith('tools-pages')
  })

  it('clears data saved for another user before claiming it', async () => {
    jest.mocked(getOwner).mockResolvedValue('someone-else')

    await claimOfflineData('me')

    expect(clearStore).toHaveBeenCalled()
    expect(setOwner).toHaveBeenCalledWith('me')
    expect(pruneEntries).toHaveBeenCalled()
  })

  it('keeps data already owned by the user', async () => {
    jest.mocked(getOwner).mockResolvedValue('me')

    await claimOfflineData('me')

    expect(clearStore).not.toHaveBeenCalled()
    expect(setOwner).not.toHaveBeenCalled()
  })

  it('claims unowned data without clearing it', async () => {
    jest.mocked(getOwner).mockResolvedValue(undefined)

    await claimOfflineData('me')

    expect(clearStore).not.toHaveBeenCalled()
    expect(setOwner).toHaveBeenCalledWith('me')
  })
})

describe('offline session without service worker support', () => {
  it('is a no-op', async () => {
    // @ts-expect-error -- simulate a browser without service workers
    delete navigator.serviceWorker
    // @ts-expect-error -- simulate a browser without CacheStorage
    delete globalThis.caches

    registerServiceWorker()
    savePageForOffline('/feeds')
    await clearOfflineData()

    expect(clearStore).toHaveBeenCalled()
  })
})
