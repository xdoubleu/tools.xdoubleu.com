'use client'

import { useProgressWebSocket } from '@/lib/backlog/progressWebSocket'

interface ProgressWidgetProps {
  wsUrl: string
}

export default function ProgressWidget({ wsUrl }: ProgressWidgetProps) {
  const { status, lastMessage } = useProgressWebSocket(wsUrl)

  const isConnected = status === WebSocket.OPEN
  const statusText = isConnected ? 'Connected' : 'Disconnected'
  const statusColor = isConnected ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'

  return (
    <div className={`p-3 rounded ${statusColor} mb-4`}>
      <div className="flex items-center justify-between">
        <span className="font-medium">{statusText}</span>
        {lastMessage && <span className="text-sm">{lastMessage}</span>}
      </div>
    </div>
  )
}
