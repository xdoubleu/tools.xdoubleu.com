export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected'
export type TrackType = 'cam' | 'screen'

export interface WsMessage {
  type: 'offer' | 'answer' | 'candidate'
  payload: RTCSessionDescriptionInit | RTCIceCandidateInit
  trackType: TrackType
  direction?: 'send' | 'recv'
}

export const STATUS_LABEL: Record<ConnectionStatus, string> = {
  connecting: 'Connecting...',
  connected: 'Connected',
  disconnected: 'Disconnected'
}

export const STATUS_COLOR: Record<ConnectionStatus, string> = {
  connecting: 'bg-yellow-400',
  connected: 'bg-green-500',
  disconnected: 'bg-red-500'
}
