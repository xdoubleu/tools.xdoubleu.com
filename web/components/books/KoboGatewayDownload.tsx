'use client'

import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { GATEWAY_DOWNLOAD_PATH } from '@/lib/books/gatewayClient'

/**
 * Download instructions for the local kobo-gateway. macOS-only (the binary
 * is darwin/arm64), so the card renders nothing on other platforms. The
 * curl route is primary because curl doesn't set the com.apple.quarantine
 * attribute, so Gatekeeper doesn't block the unsigned binary.
 */
export default function KoboGatewayDownload() {
  // Both values need the browser; setting them in an effect keeps SSR and
  // the first client render identical (the card appears after hydration).
  const [isMac, setIsMac] = useState(false)
  const [origin, setOrigin] = useState('')

  useEffect(() => {
    setIsMac(/Mac/i.test(navigator.userAgent))
    setOrigin(window.location.origin)
  }, [])

  if (!isMac) return null

  const curlCommand = `curl -fsSL ${origin}${GATEWAY_DOWNLOAD_PATH} -o kobo-gateway && chmod +x kobo-gateway && ./kobo-gateway`

  return (
    <div
      className="mt-4 space-y-2 rounded-2xl border border-border bg-surface px-4 py-3"
      data-testid="kobo-gateway-download"
    >
      <p className="text-sm font-medium">Using Safari or Firefox on a Mac?</p>
      <p className="text-xs text-muted">
        Download the gateway tool: a small helper that runs locally and lets this page configure
        your USB-connected Kobo in any browser. Run this in Terminal and leave it running:
      </p>
      <pre className="overflow-x-auto rounded-lg bg-hover px-3 py-2 text-xs">
        <code data-testid="kobo-gateway-curl">{curlCommand}</code>
      </pre>
      <p className="text-xs text-muted">
        Or download it directly — since the binary is unsigned, macOS will quarantine browser
        downloads; clear it with <code>xattr -d com.apple.quarantine kobo-gateway</code> before
        running. Apple Silicon only.
      </p>
      <Button asChild variant="secondary">
        <a href={GATEWAY_DOWNLOAD_PATH} download>
          Download kobo-gateway
        </a>
      </Button>
    </div>
  )
}
