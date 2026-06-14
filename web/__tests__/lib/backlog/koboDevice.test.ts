import { readKoboSerial, defaultDeviceName } from '@/lib/backlog/koboDevice'

// --- defaultDeviceName ---

describe('defaultDeviceName', () => {
  it('returns Kobo with last-4 suffix when serial has 4+ chars', () => {
    expect(defaultDeviceName('N418ABCD1234')).toBe('Kobo (…1234)')
  })

  it('uses exactly the last 4 chars', () => {
    expect(defaultDeviceName('WXYZ')).toBe('Kobo (…WXYZ)')
  })

  it('falls back to "My Kobo" when serial is too short', () => {
    expect(defaultDeviceName('ABC')).toBe('My Kobo')
    expect(defaultDeviceName('')).toBe('My Kobo')
  })
})

// --- readKoboSerial ---

function makeVersionHandle(content: string) {
  const mockFile = { text: jest.fn().mockResolvedValue(content) }
  return { getFile: jest.fn().mockResolvedValue(mockFile) }
}

function asRoot(obj: unknown): FileSystemDirectoryHandle {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
  return obj as FileSystemDirectoryHandle
}

function makeKoboRoot(versionContent: string) {
  const versionHandle = makeVersionHandle(versionContent)
  const koboDir = { getFileHandle: jest.fn().mockResolvedValue(versionHandle) }
  return asRoot({ getDirectoryHandle: jest.fn().mockResolvedValue(koboDir) })
}

describe('readKoboSerial', () => {
  it('extracts serial from the first comma-separated field', async () => {
    const root = makeKoboRoot('N418ABCD1234,4.38.21908,EXTRA')
    const serial = await readKoboSerial(root)
    expect(serial).toBe('N418ABCD1234')
  })

  it('returns the whole line when there is no comma', async () => {
    const root = makeKoboRoot('N418NOSERIAL')
    const serial = await readKoboSerial(root)
    expect(serial).toBe('N418NOSERIAL')
  })

  it('returns empty string when .kobo directory is missing', async () => {
    const root = asRoot({
      getDirectoryHandle: jest.fn().mockRejectedValue(new Error('Not found'))
    })
    const serial = await readKoboSerial(root)
    expect(serial).toBe('')
  })

  it('returns empty string when version file is missing', async () => {
    const koboDir = { getFileHandle: jest.fn().mockRejectedValue(new Error('No file')) }
    const root = asRoot({ getDirectoryHandle: jest.fn().mockResolvedValue(koboDir) })
    const serial = await readKoboSerial(root)
    expect(serial).toBe('')
  })
})
