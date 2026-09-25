export function buildPresenterUrl(baseUrl: string, roomId: string): string {
  const base = baseUrl.replace(/\/$/, '')
  return `${base}/watchparty/${roomId}/presenter`
}

export function buildViewerUrl(baseUrl: string, roomId: string): string {
  const base = baseUrl.replace(/\/$/, '')
  return `${base}/watchparty/${roomId}`
}

/** WebSocket signaling URL; one endpoint serves both roles (sent in the first message). */
export function buildWsUrl(apiUrl: string, roomId: string, isPresenter: boolean): string {
  const wsBase = (apiUrl ?? '')
    .replace(/^https:\/\//, 'wss://')
    .replace(/^http:\/\//, 'ws://')
    .replace(/\/$/, '')
  const role = isPresenter ? 'presenter' : 'viewer'
  return `${wsBase}/watchparty/api/signaling?roomCode=${encodeURIComponent(roomId)}&role=${role}`
}

/** Makes an element draggable within its positioned parent; returns a cleanup. */
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

/** Room ID from /watchparty/<id>[/presenter], or null. */
export function parseRoomId(path: string): string | null {
  const match = path.match(/\/watchparty\/([^/]+)(?:\/|$)/)
  if (!match) return null
  const id = match[1]
  if (!id) return null
  return id
}
