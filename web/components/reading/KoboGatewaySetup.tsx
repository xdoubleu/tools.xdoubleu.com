'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { getApiUrl } from '@/lib/env'
import { KOBO_DEFAULT_ENDPOINT, isManagedEndpoint } from '@/lib/reading/koboConf'
import { defaultDeviceName } from '@/lib/reading/koboDevice'
import {
  configureGateway,
  gatewayNeedsUpdate,
  revertGateway,
  updateGateway,
  type GatewayStatus,
  type GatewayKobo
} from '@/lib/reading/gatewayClient'
import { useGatewayStatus } from '@/hooks/useKoboGateway'
import {
  useRegisterKoboDevice,
  useDisconnectKoboDevice,
  useListKoboDevices
} from '@/hooks/useBooks'

type GatewayState = 'idle' | 'updating' | 'configuring' | 'success' | 'reverting' | 'error'

const UPDATE_POLL_ATTEMPTS = 10

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

interface KoboGatewaySetupProps {
  status: GatewayStatus
  /** Delay between /status polls while the gateway restarts after an update. */
  pollIntervalMs?: number
}

/**
 * Gateway-driven Kobo setup: the local kobo-gateway does the file work while
 * this component keeps making the authenticated API calls. Gateway
 * reachability and the connected-Kobo list come from useGatewayStatus's
 * background polling — there's no manual re-check, the UI just updates.
 */
export default function KoboGatewaySetup({ status, pollIntervalMs = 1500 }: KoboGatewaySetupProps) {
  const { mutate: mutateGatewayStatus } = useGatewayStatus()
  const [state, setState] = useState<GatewayState>('idle')
  const [error, setError] = useState<string | null>(null)
  const [selectedVolume, setSelectedVolume] = useState<string | null>(null)
  const [originalEndpoint, setOriginalEndpoint] = useState<string | null>(null)
  const [deviceId, setDeviceId] = useState<string | null>(null)
  // Guards against re-triggering the self-update on every poll tick.
  const updateAttempted = useRef(false)

  const registerKoboDevice = useRegisterKoboDevice()
  const disconnectKoboDevice = useDisconnectKoboDevice()
  const { data: devices, mutate: mutateDevices } = useListKoboDevices()

  const runUpdate = useCallback(async () => {
    try {
      await updateGateway()
      for (let attempt = 0; attempt < UPDATE_POLL_ATTEMPTS; attempt++) {
        await sleep(pollIntervalMs)
        const fresh = await mutateGatewayStatus()
        if (fresh && !gatewayNeedsUpdate(fresh)) {
          setState('idle')
          return
        }
      }
      throw new Error('The gateway did not come back after updating.')
    } catch (err: unknown) {
      const msg =
        err instanceof Error && err.message ? err.message : 'Failed to update the gateway.'
      setError(`${msg} Download the latest version manually and start it again.`)
      setState('error')
    }
  }, [pollIntervalMs, mutateGatewayStatus])

  useEffect(() => {
    if (gatewayNeedsUpdate(status) && !updateAttempted.current) {
      updateAttempted.current = true
      setState('updating')
      void runUpdate()
    }
  }, [status, runUpdate])

  async function handleConfigure(kobo: GatewayKobo) {
    setState('configuring')
    setError(null)

    try {
      const res = await registerKoboDevice(defaultDeviceName(kobo.serial), kobo.serial)
      const { originalEndpoint: orig } = await configureGateway(
        `${getApiUrl()}/reading/kobo/${res.rawToken}`,
        kobo.volumePath
      )

      setOriginalEndpoint(orig)
      setDeviceId(res.device?.id ?? '')
      setState('success')
      await mutateDevices()
      await mutateGatewayStatus()
    } catch (err: unknown) {
      setError(err instanceof Error && err.message ? err.message : 'Failed to configure Kobo.')
      setState('error')
    }
  }

  /**
   * Reverts the conf via the gateway and revokes the matching device token.
   * Mirrors handleRevert in KoboSetup: an explicit device ID (post-configure)
   * wins; otherwise the device is matched by serial.
   */
  async function handleRevert(
    kobo: GatewayKobo,
    targetEndpoint: string,
    deviceIdToRevoke: string | null
  ) {
    setState('reverting')
    setError(null)

    try {
      const { serial } = await revertGateway(targetEndpoint, kobo.volumePath)

      let idToRevoke = deviceIdToRevoke
      if (!idToRevoke) {
        const match = devices?.devices.find((d) => d.serial === serial)
        idToRevoke = match?.id ?? null
      }
      if (idToRevoke) {
        await disconnectKoboDevice(idToRevoke)
      }

      setOriginalEndpoint(null)
      setDeviceId(null)
      setState('idle')
      await mutateDevices()
      await mutateGatewayStatus()
    } catch (err: unknown) {
      setError(err instanceof Error && err.message ? err.message : 'Failed to revert.')
      setState('error')
    }
  }

  if (state === 'updating') {
    return (
      <p className="text-sm text-muted" data-testid="kobo-gateway-updating">
        Updating the gateway to the latest version…
      </p>
    )
  }

  const kobos = status.kobos
  const kobo =
    (selectedVolume && kobos.find((k) => k.volumePath === selectedVolume)) ||
    (kobos.length === 1 ? kobos[0] : null)

  return (
    <div className="space-y-3" data-testid="kobo-gateway-setup">
      <p className="text-xs text-muted" data-testid="kobo-gateway-detected">
        Local gateway detected — your Kobo is configured directly over USB.
      </p>

      {kobos.length === 0 && state !== 'error' && (
        <p className="text-sm text-muted" data-testid="kobo-gateway-no-kobo">
          No Kobo detected. Plug it in via USB — this updates automatically.
        </p>
      )}

      {kobos.length > 1 && !kobo && (
        <div className="space-y-2" data-testid="kobo-gateway-picker">
          <p className="text-sm text-muted">Multiple Kobos found — pick one:</p>
          {kobos.map((k) => (
            <Button
              key={k.volumePath}
              type="button"
              variant="secondary"
              onClick={() => setSelectedVolume(k.volumePath)}
            >
              {k.volumePath} {k.serial && `(${defaultDeviceName(k.serial)})`}
            </Button>
          ))}
        </div>
      )}

      {kobo && state === 'idle' && !isManagedEndpoint(kobo.currentEndpoint, getApiUrl()) && (
        <Button
          type="button"
          onClick={() => handleConfigure(kobo)}
          data-testid="kobo-gateway-configure-btn"
        >
          Configure {defaultDeviceName(kobo.serial)}
        </Button>
      )}

      {kobo && state === 'idle' && isManagedEndpoint(kobo.currentEndpoint, getApiUrl()) && (
        <div className="space-y-2">
          <div
            className="rounded-xl border border-success/30 bg-success/10 px-4 py-3 text-sm text-success"
            data-testid="kobo-gateway-already-configured"
          >
            This Kobo is already configured for sync with this server.
          </div>
          <Button
            type="button"
            variant="secondary"
            onClick={() => handleRevert(kobo, KOBO_DEFAULT_ENDPOINT, null)}
            data-testid="kobo-gateway-revert-btn"
          >
            Revert to original Kobo settings
          </Button>
        </div>
      )}

      {state === 'configuring' && (
        <Button type="button" disabled data-testid="kobo-gateway-configure-btn">
          Configuring…
        </Button>
      )}

      {state === 'reverting' && (
        <Button type="button" disabled data-testid="kobo-gateway-revert-btn">
          Reverting…
        </Button>
      )}

      {kobo && state === 'success' && (
        <div className="space-y-2">
          <p className="text-sm" data-testid="kobo-gateway-success">
            Kobo configured. Safely eject and reconnect your Kobo to start syncing.
          </p>
          <Button
            type="button"
            variant="secondary"
            onClick={() => handleRevert(kobo, originalEndpoint || KOBO_DEFAULT_ENDPOINT, deviceId)}
            data-testid="kobo-gateway-revert-btn"
          >
            Revert configuration
          </Button>
        </div>
      )}

      {error && (
        <div className="space-y-2">
          <p className="text-sm text-danger" data-testid="kobo-gateway-error">
            {error}
          </p>
          <Button
            type="button"
            variant="secondary"
            onClick={() => {
              setError(null)
              setState('idle')
            }}
          >
            Dismiss
          </Button>
        </div>
      )}
    </div>
  )
}
