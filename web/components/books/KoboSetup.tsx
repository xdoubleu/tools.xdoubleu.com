'use client'

// showDirectoryPicker is not yet in all TS DOM lib versions; declare it here.
declare global {
  function showDirectoryPicker(options?: {
    mode?: 'read' | 'readwrite'
    startIn?: string
  }): Promise<FileSystemDirectoryHandle>

  interface Window {
    showDirectoryPicker?: typeof showDirectoryPicker
  }
}

import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import KoboGatewaySetup from '@/components/books/KoboGatewaySetup'
import { probeGateway, type GatewayStatus } from '@/lib/books/gatewayClient'
import { getApiUrl } from '@/lib/env'
import {
  parseKoboConf,
  serializeKoboConf,
  patchApiEndpoint,
  revertApiEndpoint,
  getApiEndpoint,
  isManagedEndpoint,
  KOBO_DEFAULT_ENDPOINT
} from '@/lib/books/koboConf'
import { readKoboSerial, defaultDeviceName } from '@/lib/books/koboDevice'
import {
  useRegisterKoboDevice,
  useDisconnectKoboDevice,
  useListKoboDevices
} from '@/hooks/useBooks'

type SetupState =
  | 'idle'
  | 'detecting'
  | 'already-configured'
  | 'configuring'
  | 'success'
  | 'reverting'
  | 'error'

function koboSyncUrl(rawToken: string): string {
  return `${getApiUrl()}/books/kobo/${rawToken}`
}

async function readKoboConf(root: FileSystemDirectoryHandle) {
  const koboDir = await root.getDirectoryHandle('.kobo')
  const innerDir = await koboDir.getDirectoryHandle('Kobo')
  const fileHandle = await innerDir.getFileHandle('Kobo eReader.conf')
  const file = await fileHandle.getFile()
  const raw = await file.text()
  return { conf: parseKoboConf(raw), fileHandle }
}

export default function KoboSetup() {
  const [state, setState] = useState<SetupState>('idle')
  const [error, setError] = useState<string | null>(null)
  const [originalEndpoint, setOriginalEndpoint] = useState<string | null>(null)
  const [rootHandle, setRootHandle] = useState<FileSystemDirectoryHandle | null>(null)
  // The device ID returned by RegisterKoboDevice, needed for revert-and-revoke.
  const [deviceId, setDeviceId] = useState<string | null>(null)

  // When the local kobo-gateway responds it takes over the whole flow: it
  // does the file work natively, so setup also works in Safari/Firefox.
  const [gatewayStatus, setGatewayStatus] = useState<GatewayStatus | null>(null)

  const registerKoboDevice = useRegisterKoboDevice()
  const disconnectKoboDevice = useDisconnectKoboDevice()
  const { data: devices, mutate: mutateDevices } = useListKoboDevices()

  useEffect(() => {
    let cancelled = false
    void probeGateway().then((status) => {
      if (!cancelled && status) setGatewayStatus(status)
    })
    return () => {
      cancelled = true
    }
  }, [])

  async function handleGatewayRecheck() {
    const status = await probeGateway()
    if (status) setGatewayStatus(status)
  }

  const fsSupported =
    typeof window !== 'undefined' && typeof window.showDirectoryPicker === 'function'

  if (gatewayStatus) {
    return <KoboGatewaySetup initialStatus={gatewayStatus} />
  }

  if (!fsSupported) {
    return <KoboFallback onGatewayRecheck={handleGatewayRecheck} />
  }

  async function handleDetect() {
    setState('detecting')
    setError(null)

    try {
      const root = await showDirectoryPicker({ mode: 'readwrite' })
      const { conf } = await readKoboConf(root)
      const endpoint = getApiEndpoint(conf)

      if (isManagedEndpoint(endpoint, getApiUrl())) {
        // Device is already configured for this server.
        setRootHandle(root)
        setState('already-configured')
        return
      }

      // Not yet configured — proceed to register + write.
      setState('configuring')
      const serial = await readKoboSerial(root)
      const name = defaultDeviceName(serial)

      const res = await registerKoboDevice(name, serial)
      const rawToken = res.rawToken
      const deviceID = res.device?.id ?? ''

      const { conf: currentConf, fileHandle } = await readKoboConf(root)
      const newUrl = koboSyncUrl(rawToken)
      const { conf: patched, originalEndpoint: orig } = patchApiEndpoint(currentConf, newUrl)

      const writable = await fileHandle.createWritable()
      await writable.write(serializeKoboConf(patched))
      await writable.close()

      setOriginalEndpoint(orig)
      setRootHandle(root)
      setDeviceId(deviceID)
      setState('success')
      await mutateDevices()
    } catch (err: unknown) {
      if (err instanceof Error && err.name === 'AbortError') {
        setState('idle')
        return
      }
      const msg =
        err instanceof Error && err.message
          ? err.message
          : 'Could not configure Kobo. Make sure you selected the Kobo drive root.'
      setError(msg)
      setState('error')
    }
  }

  /**
   * Revert the Kobo conf to `targetEndpoint` and revoke the server-side token.
   *
   * When `deviceIdToRevoke` is supplied (post-configure success path), that ID
   * is used directly. For the already-configured path we don't have the ID in
   * state, so we read the serial from the device and look up the matching entry
   * in the devices list. The conf is always reverted; the disconnect is skipped
   * only when no matching device is found (e.g. already revoked).
   */
  async function handleRevert(targetEndpoint: string, deviceIdToRevoke: string | null) {
    if (!rootHandle) return
    setState('reverting')
    setError(null)

    try {
      const { conf, fileHandle } = await readKoboConf(rootHandle)
      const reverted = revertApiEndpoint(conf, targetEndpoint)

      const writable = await fileHandle.createWritable()
      await writable.write(serializeKoboConf(reverted))
      await writable.close()

      // Determine which device token to revoke.
      let idToRevoke = deviceIdToRevoke
      if (!idToRevoke) {
        const serial = await readKoboSerial(rootHandle)
        const match = devices?.devices.find((d) => d.serial === serial)
        idToRevoke = match?.id ?? null
      }

      if (idToRevoke) {
        await disconnectKoboDevice(idToRevoke)
      }

      setOriginalEndpoint(null)
      setRootHandle(null)
      setDeviceId(null)
      setState('idle')
      await mutateDevices()
    } catch (err: unknown) {
      const msg = err instanceof Error && err.message ? err.message : 'Failed to revert.'
      setError(msg)
      setState('error')
    }
  }

  return (
    <div className="space-y-3" data-testid="kobo-setup">
      {(state === 'idle' || state === 'error') && (
        <Button type="button" onClick={handleDetect} data-testid="kobo-detect-btn">
          Select my Kobo
        </Button>
      )}

      {state === 'detecting' && (
        <Button type="button" disabled data-testid="kobo-detect-btn">
          Selecting…
        </Button>
      )}

      {state === 'configuring' && (
        <Button type="button" disabled data-testid="kobo-configure-btn">
          Configuring…
        </Button>
      )}

      {state === 'reverting' && (
        <Button type="button" disabled data-testid="kobo-revert-btn">
          Reverting…
        </Button>
      )}

      {state === 'already-configured' && (
        <div className="space-y-2">
          <div
            className="rounded-xl border border-success/30 bg-success/10 px-4 py-3 text-sm text-success"
            data-testid="kobo-already-configured"
          >
            This Kobo is already configured for sync with this server.
          </div>
          <Button
            type="button"
            variant="secondary"
            onClick={() => handleRevert(KOBO_DEFAULT_ENDPOINT, null)}
            data-testid="kobo-revert-btn"
          >
            Revert to original Kobo settings
          </Button>
        </div>
      )}

      {state === 'success' && (
        <div className="space-y-2">
          <p className="text-sm" data-testid="kobo-setup-success">
            Kobo configured. Safely eject and reconnect your Kobo to start syncing.
          </p>
          <Button
            type="button"
            variant="secondary"
            onClick={() => handleRevert(originalEndpoint!, deviceId)}
            data-testid="kobo-revert-btn"
          >
            Revert configuration
          </Button>
        </div>
      )}

      {error && (
        <p className="text-sm text-danger" data-testid="kobo-setup-error">
          {error}
        </p>
      )}
    </div>
  )
}

function KoboFallback({ onGatewayRecheck }: { onGatewayRecheck: () => Promise<void> }) {
  return (
    <div
      className="space-y-2 rounded-xl border border-border bg-surface px-4 py-3"
      data-testid="kobo-setup-fallback"
    >
      <p className="text-sm text-subtle">
        Your browser does not support automatic Kobo setup. On a Mac, download and run the gateway
        tool below — or follow these steps manually:
      </p>
      <ol className="list-decimal list-inside space-y-1 text-xs text-muted">
        <li>Connect your Kobo via USB so it appears as a drive.</li>
        <li>
          Open <code>.kobo/Kobo/Kobo eReader.conf</code> in a text editor.
        </li>
        <li>
          Under <code>[OneStoreServices]</code>, set <code>api_endpoint</code> to the URL shown in
          the connected devices list below (register a device first if none appear).
        </li>
        <li>Save the file, eject, and reconnect your Kobo to sync.</li>
      </ol>
      <Button
        type="button"
        variant="secondary"
        onClick={onGatewayRecheck}
        data-testid="kobo-gateway-recheck-btn"
      >
        I started the gateway — re-check
      </Button>
    </div>
  )
}
