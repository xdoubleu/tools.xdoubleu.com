'use client'

import { useProgressWebSocket } from '@/lib/backlog/progressWebSocket'

interface ProgressWidgetProps {
  wsUrl: string
}

export default function ProgressWidget({ wsUrl }: ProgressWidgetProps) {
  const { status, lastMessage } = useProgressWebSocket(wsUrl)

  const isConnected = status === WebSocket.OPEN

  return (
    <div
      className={[
        'mb-4 rounded-2xl border px-4 py-3',
        isConnected
          ? 'border-success/30 bg-success/10 text-success'
          : 'border-danger/30 bg-danger/10 text-danger'
      ].join(' ')}
    >
      <div className="flex items-center justify-between">
        <span className="font-medium text-sm">{isConnected ? 'Connected' : 'Disconnected'}</span>
        {lastMessage && <span className="text-sm">{lastMessage}</span>}
      </div>
    </div>
  )
}
