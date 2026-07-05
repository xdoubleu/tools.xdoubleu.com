import {
  buildPresenterUrl,
  buildViewerUrl,
  buildWsUrl,
  parseRoomId,
  attachDraggable
} from '../../../lib/watchparty/roomUtils'
import { STATUS_LABEL, STATUS_COLOR } from '../../../lib/watchparty/types'

// jsdom does not define PointerEvent
if (typeof PointerEvent === 'undefined') {
  class PointerEventPolyfill extends MouseEvent {
    pointerId: number
    constructor(type: string, params: PointerEventInit = {}) {
      super(type, params)
      this.pointerId = params.pointerId ?? 0
    }
  }
  Object.defineProperty(global, 'PointerEvent', {
    value: PointerEventPolyfill,
    writable: true,
    configurable: true
  })
}

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

describe('attachDraggable', () => {
  function makeEl() {
    const container = document.createElement('div')
    container.style.position = 'relative'
    document.body.appendChild(container)
    const el = document.createElement('div')
    container.appendChild(el)
    jest.spyOn(container, 'getBoundingClientRect').mockReturnValue({
      left: 0,
      top: 0,
      width: 200,
      height: 200,
      right: 200,
      bottom: 200,
      x: 0,
      y: 0,
      toJSON: () => ({})
    })
    jest.spyOn(el, 'getBoundingClientRect').mockReturnValue({
      left: 10,
      top: 10,
      width: 50,
      height: 50,
      right: 60,
      bottom: 60,
      x: 10,
      y: 10,
      toJSON: () => ({})
    })
    // jsdom does not implement pointer capture — stub the methods directly
    el.setPointerCapture = jest.fn()
    el.hasPointerCapture = jest.fn().mockReturnValue(true)
    el.releasePointerCapture = jest.fn()
    return { el, container }
  }

  it('returns a cleanup function', () => {
    const { el, container } = makeEl()
    const cleanup = attachDraggable(el)
    expect(typeof cleanup).toBe('function')
    cleanup()
    document.body.removeChild(container)
  })

  it('attaches three pointer event listeners', () => {
    const { el, container } = makeEl()
    const addSpy = jest.spyOn(el, 'addEventListener')
    const cleanup = attachDraggable(el)
    const eventNames = addSpy.mock.calls.map((c) => c[0])
    expect(eventNames).toContain('pointerdown')
    expect(eventNames).toContain('pointermove')
    expect(eventNames).toContain('pointerup')
    cleanup()
    document.body.removeChild(container)
  })

  it('removes listeners on cleanup', () => {
    const { el, container } = makeEl()
    const removeSpy = jest.spyOn(el, 'removeEventListener')
    const cleanup = attachDraggable(el)
    cleanup()
    expect(removeSpy).toHaveBeenCalledTimes(3)
    document.body.removeChild(container)
  })

  it('pointerdown sets cursor to grabbing and records start position', () => {
    const { el, container } = makeEl()
    attachDraggable(el)
    el.dispatchEvent(new PointerEvent('pointerdown', { clientX: 20, clientY: 30, pointerId: 1 }))
    expect(el.style.cursor).toBe('grabbing')
    expect(el.style.left).toBe('10px')
    expect(el.style.top).toBe('10px')
    document.body.removeChild(container)
  })

  it('pointermove updates position when pointer is captured', () => {
    const { el, container } = makeEl()
    attachDraggable(el)
    el.dispatchEvent(new PointerEvent('pointerdown', { clientX: 20, clientY: 30, pointerId: 1 }))
    el.dispatchEvent(new PointerEvent('pointermove', { clientX: 25, clientY: 35, pointerId: 1 }))
    expect(el.style.left).toBe('15px')
    expect(el.style.top).toBe('15px')
    document.body.removeChild(container)
  })

  it('pointerup releases capture and sets cursor to grab', () => {
    const { el, container } = makeEl()
    attachDraggable(el)
    el.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1 }))
    expect(el.releasePointerCapture).toHaveBeenCalled()
    expect(el.style.cursor).toBe('grab')
    document.body.removeChild(container)
  })
})

describe('STATUS_LABEL and STATUS_COLOR constants', () => {
  it('STATUS_LABEL has entries for all statuses', () => {
    expect(STATUS_LABEL.connecting).toBe('Connecting…')
    expect(STATUS_LABEL.connected).toBe('Connected')
    expect(STATUS_LABEL.disconnected).toBe('Disconnected')
  })

  it('STATUS_COLOR has entries for all statuses', () => {
    expect(STATUS_COLOR.connecting).toContain('yellow')
    expect(STATUS_COLOR.connected).toContain('green')
    expect(STATUS_COLOR.disconnected).toContain('red')
  })
})
