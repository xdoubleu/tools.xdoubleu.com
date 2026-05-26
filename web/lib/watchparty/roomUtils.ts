/**
 * Build the presenter page URL for a room.
 */
export function buildPresenterUrl(baseUrl: string, roomId: string): string {
  const base = baseUrl.replace(/\/$/, '')
  return `${base}/watchparty/${roomId}/presenter`
}

/**
 * Build the viewer page URL for a room.
 */
export function buildViewerUrl(baseUrl: string, roomId: string): string {
  const base = baseUrl.replace(/\/$/, '')
  return `${base}/watchparty/${roomId}`
}

/**
 * Build the WebSocket signaling URL.
 * Replaces http:// with ws:// and https:// with wss://.
 * The single /watchparty/api/signaling endpoint handles both roles
 * (role is sent in the first WS message).
 */
export function buildWsUrl(apiUrl: string, roomId: string, isPresenter: boolean): string {
  const wsBase = (apiUrl ?? '')
    .replace(/^https:\/\//, 'wss://')
    .replace(/^http:\/\//, 'ws://')
    .replace(/\/$/, '')
  // roomId is included as a query parameter for routing context;
  // the actual role is negotiated via the first WS message.
  const role = isPresenter ? 'presenter' : 'viewer'
  return `${wsBase}/watchparty/api/signaling?roomCode=${encodeURIComponent(roomId)}&role=${role}`
}

/**
 * Make a video element draggable within its positioned parent.
 * Returns a cleanup function to remove the event listeners.
 */
export function attachDraggable(el: HTMLElement): () => void {
  let startX = 0,
    startY = 0,
    origLeft = 0,
    origTop = 0

  const onDown = (e: PointerEvent) => {
    e.preventDefault()
    el.setPointerCapture(e.pointerId)
    el.style.cursor = 'grabbing'
    const container = el.parentElement!.getBoundingClientRect()
    const rect = el.getBoundingClientRect()
    el.style.right = 'auto'
    el.style.bottom = 'auto'
    el.style.left = `${rect.left - container.left}px`
    el.style.top = `${rect.top - container.top}px`
    startX = e.clientX
    startY = e.clientY
    origLeft = parseFloat(el.style.left)
    origTop = parseFloat(el.style.top)
  }
  const onMove = (e: PointerEvent) => {
    if (!el.hasPointerCapture(e.pointerId)) return
    el.style.left = `${origLeft + e.clientX - startX}px`
    el.style.top = `${origTop + e.clientY - startY}px`
  }
  const onUp = (e: PointerEvent) => {
    el.releasePointerCapture(e.pointerId)
    el.style.cursor = 'grab'
  }

  el.addEventListener('pointerdown', onDown)
  el.addEventListener('pointermove', onMove)
  el.addEventListener('pointerup', onUp)
  return () => {
    el.removeEventListener('pointerdown', onDown)
    el.removeEventListener('pointermove', onMove)
    el.removeEventListener('pointerup', onUp)
  }
}

/**
 * Extract the room ID from a URL path like /watchparty/abc123 or /watchparty/abc123/presenter.
 * Returns null if the path does not match.
 */
export function parseRoomId(path: string): string | null {
  const match = path.match(/\/watchparty\/([^/]+)(?:\/|$)/)
  if (!match) return null
  const id = match[1]
  // Guard against empty segments
  if (!id) return null
  return id
}
