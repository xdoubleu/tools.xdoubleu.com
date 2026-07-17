'use client'

import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { GATEWAY_DOWNLOAD_PATH } from '@/lib/reading/gatewayClient'

/**
 * Prompts for the kobo-gateway menu-bar app — the only way to set up a
 * Kobo (there is no in-browser fallback). macOS-only (the app is
 * darwin/arm64); other platforms get a short explanatory note instead.
 */
export default function KoboGatewayDownload() {
  // Read in an effect so SSR and the first client render stay identical.
  const [isMac, setIsMac] = useState<boolean | null>(null)

  useEffect(() => {
    setIsMac(/Mac/i.test(navigator.userAgent))
  }, [])

  if (isMac === null) return null

  if (!isMac) {
    return (
      <div
        className="space-y-2 rounded-2xl border border-border bg-surface px-4 py-3"
        data-testid="kobo-gateway-non-mac"
      >
        <p className="text-sm text-muted">
          Kobo setup requires the kobo-gateway app, which is only available for Apple Silicon Macs.
        </p>
      </div>
    )
  }

  return (
    <div
      className="space-y-2 rounded-2xl border border-border bg-surface px-4 py-3"
      data-testid="kobo-gateway-download"
    >
      <p className="text-sm font-medium">Set up your Kobo</p>
      <p className="text-xs text-muted">
        Download the gateway app: a small menu-bar helper that lets this page configure your
        USB-connected Kobo. Open the downloaded file, drag it into Applications, then launch it — a
        Kobo icon appears in the menu bar while it&apos;s running.
      </p>
      <Button asChild>
        <a href={GATEWAY_DOWNLOAD_PATH} download>
          Download Kobo Gateway
        </a>
      </Button>
      <p className="text-xs text-muted">
        The app is unsigned, so macOS will block it the first time: right-click (or Control-click)
        it in Applications and choose <strong>Open</strong>, then confirm in the dialog that
        appears. If macOS still refuses, go to System Settings → Privacy &amp; Security and click{' '}
        <strong>Open Anyway</strong> next to the Kobo Gateway warning.
      </p>
    </div>
  )
}
