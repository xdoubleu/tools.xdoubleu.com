'use client'

import { useState } from 'react'
import { useListKoboDevices, useDisconnectKoboDevice } from '@/hooks/useBooks'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { formatDate } from '@/lib/dates'

function formatLastSeen(lastSeenAt: string): string {
  if (!lastSeenAt) return 'Never synced'
  return `Last synced ${formatDate(lastSeenAt)}`
}

interface DisconnectDialogProps {
  open: boolean
  deviceName: string
  onCancel: () => void
  onConfirm: () => Promise<void>
}

function DisconnectDialog({ open, deviceName, onCancel, onConfirm }: DisconnectDialogProps) {
  const [disconnecting, setDisconnecting] = useState(false)
  const [error, setError] = useState('')

  async function handleConfirm() {
    setDisconnecting(true)
    setError('')
    try {
      await onConfirm()
    } catch {
      setError('Failed to disconnect device. Please try again.')
      setDisconnecting(false)
    }
  }

  function handleCancel() {
    if (!disconnecting) {
      setError('')
      onCancel()
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && handleCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Disconnect {deviceName}</DialogTitle>
        </DialogHeader>
        <p className="text-sm text-muted">
          This will revoke the device&apos;s sync token. The Kobo will no longer be able to sync
          until you reconfigure it.
        </p>
        {error && (
          <p className="mt-2 text-sm text-danger" data-testid="disconnect-error">
            {error}
          </p>
        )}
        <div className="mt-6 flex justify-end gap-2">
          <Button variant="ghost" disabled={disconnecting} onClick={handleCancel}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={disconnecting}
            onClick={handleConfirm}
            data-testid="disconnect-confirm-btn"
          >
            {disconnecting ? 'Disconnecting…' : 'Disconnect'}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

export default function KoboDevices() {
  const { data, isLoading, mutate } = useListKoboDevices()
  const disconnectKoboDevice = useDisconnectKoboDevice()

  const [pendingId, setPendingId] = useState<string | null>(null)
  const [pendingName, setPendingName] = useState('')

  const devices = data?.devices ?? []

  async function handleDisconnect() {
    if (!pendingId) return
    await disconnectKoboDevice(pendingId)
    await mutate()
    setPendingId(null)
  }

  if (isLoading) {
    return (
      <p className="text-xs text-muted" data-testid="kobo-devices-loading">
        Loading devices…
      </p>
    )
  }

  if (devices.length === 0) {
    return (
      <p className="text-xs text-muted" data-testid="kobo-devices-empty">
        No devices connected yet. Use the button above to configure your Kobo.
      </p>
    )
  }

  return (
    <>
      <ul className="space-y-2" data-testid="kobo-devices-list">
        {devices.map((device) => (
          <li
            key={device.id}
            className="flex items-center justify-between rounded-xl border border-border bg-card px-4 py-3"
            data-testid={`kobo-device-${device.id}`}
          >
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">{device.name}</p>
              <p className="text-xs text-muted">
                {device.serial ? `Serial: ${device.serial} · ` : ''}
                {formatLastSeen(device.lastSeenAt)}
              </p>
            </div>
            <Button
              type="button"
              variant="destructive"
              size="sm"
              className="ml-4 shrink-0"
              onClick={() => {
                setPendingId(device.id)
                setPendingName(device.name)
              }}
              data-testid={`kobo-disconnect-btn-${device.id}`}
            >
              Disconnect
            </Button>
          </li>
        ))}
      </ul>

      <DisconnectDialog
        open={pendingId !== null}
        deviceName={pendingName}
        onCancel={() => setPendingId(null)}
        onConfirm={handleDisconnect}
      />
    </>
  )
}
