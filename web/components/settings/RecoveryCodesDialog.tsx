'use client'

import { useState } from 'react'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'

interface RecoveryCodesDialogProps {
  codes: string[]
  onDismiss: () => void
}

// One-time screen for the recovery codes MFAEnrollVerify/RegenerateRecoveryCodes
// return — the api never surfaces these again after this response, so the
// user must explicitly confirm they've saved them before the dialog closes.
// Deliberately has no onOpenChange/backdrop-click dismissal: the only way out
// is the "I've saved these" confirmation below.
export default function RecoveryCodesDialog({ codes, onDismiss }: RecoveryCodesDialogProps) {
  const [copied, setCopied] = useState(false)
  const [confirmed, setConfirmed] = useState(false)

  const copyCodes = async () => {
    await navigator.clipboard.writeText(codes.join('\n'))
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    // open is a fixed literal (not tied to any state we update), and
    // onOpenChange is a no-op — so an outside click or Escape (Radix's
    // default dismissal, which this component doesn't expose an override
    // for) can't actually close this dialog; the "Done" button below is the
    // only way out.
    <Dialog open modal onOpenChange={() => {}}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Save your recovery codes</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          <p className="text-sm text-muted">
            Each code can be used once to sign in if you lose access to your authenticator app. They
            won&apos;t be shown again — save them somewhere safe.
          </p>

          <div className="grid grid-cols-2 gap-2 rounded-xl border border-border bg-background p-4 font-mono text-sm">
            {codes.map((code) => (
              <span key={code}>{code}</span>
            ))}
          </div>

          <Button variant="secondary" size="sm" onClick={copyCodes}>
            {copied ? 'Copied!' : 'Copy codes'}
          </Button>

          <label className="flex items-start gap-2 text-sm text-subtle">
            <Checkbox
              checked={confirmed}
              onChange={(e) => setConfirmed(e.target.checked)}
              className="mt-1"
            />
            I&apos;ve saved these recovery codes somewhere safe.
          </label>

          <Button className="w-full" disabled={!confirmed} onClick={onDismiss}>
            Done
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
