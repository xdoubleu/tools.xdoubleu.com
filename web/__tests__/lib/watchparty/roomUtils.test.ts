import {
  buildPresenterUrl,
  buildViewerUrl,
  buildWsUrl,
  parseRoomId,
} from '../../../lib/watchparty/roomUtils'

describe('buildPresenterUrl', () => {
  it('constructs the presenter URL correctly', () => {
    expect(buildPresenterUrl('http://localhost:3000', 'abc123')).toBe(
      'http://localhost:3000/watchparty/abc123/presenter'
    )
  })

  it('strips trailing slash from baseUrl', () => {
    expect(buildPresenterUrl('http://localhost:3000/', 'room42')).toBe(
      'http://localhost:3000/watchparty/room42/presenter'
    )
  })
})

describe('buildViewerUrl', () => {
  it('constructs the viewer URL correctly', () => {
    expect(buildViewerUrl('http://localhost:3000', 'abc123')).toBe(
      'http://localhost:3000/watchparty/abc123'
    )
  })

  it('strips trailing slash from baseUrl', () => {
    expect(buildViewerUrl('https://tools.example.com/', 'xyz')).toBe(
      'https://tools.example.com/watchparty/xyz'
    )
  })
})

describe('buildWsUrl', () => {
  it('uses wss:// for https:// API URLs', () => {
    const url = buildWsUrl('https://api.example.com', 'room1', false)
    expect(url).toMatch(/^wss:\/\//)
  })

  it('uses ws:// for http:// API URLs', () => {
    const url = buildWsUrl('http://localhost:8000', 'room1', false)
    expect(url).toMatch(/^ws:\/\//)
  })

  it('includes the room code in the URL', () => {
    const url = buildWsUrl('http://localhost:8000', 'myroom', true)
    expect(url).toContain('roomCode=myroom')
  })

  it('sets role=presenter when isPresenter is true', () => {
    const url = buildWsUrl('http://localhost:8000', 'r1', true)
    expect(url).toContain('role=presenter')
  })

  it('sets role=viewer when isPresenter is false', () => {
    const url = buildWsUrl('http://localhost:8000', 'r1', false)
    expect(url).toContain('role=viewer')
  })

  it('targets the signaling endpoint', () => {
    const url = buildWsUrl('http://localhost:8000', 'r1', false)
    expect(url).toContain('/watchparty/api/signaling')
  })
})

describe('parseRoomId', () => {
  it('extracts the room ID from a viewer path', () => {
    expect(parseRoomId('/watchparty/abc123')).toBe('abc123')
  })

  it('extracts the room ID from a presenter path', () => {
    expect(parseRoomId('/watchparty/abc123/presenter')).toBe('abc123')
  })

  it('returns null for an unrelated path', () => {
    expect(parseRoomId('/todos/task/123')).toBeNull()
  })

  it('returns null for an empty string', () => {
    expect(parseRoomId('')).toBeNull()
  })

  it('returns null for path with no room segment', () => {
    expect(parseRoomId('/watchparty/')).toBeNull()
  })
})
