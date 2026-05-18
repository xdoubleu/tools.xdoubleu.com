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
export function buildWsUrl(
  apiUrl: string,
  roomId: string,
  isPresenter: boolean
): string {
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
