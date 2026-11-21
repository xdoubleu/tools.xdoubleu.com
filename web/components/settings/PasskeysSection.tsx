import { useState } from 'react'
import { Code, ConnectError } from '@connectrpc/connect'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/dialog'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { SettingsSection } from '@/components/ui/settings-section'
import { EmptyState } from '@/components/ui/states'
import {
  passkeysSupported,
  usePasskeys,
  useRegisterPasskey,
  useRemovePasskey,
  useRenamePasskey,
  useSignOut
} from '@/hooks/useAuth'
import { useRouter } from '@/hooks/navigation'
import { formatDate } from '@/lib/dates'
import type { Passkey } from '@/lib/gen/auth/v1/auth_pb'

const REAUTH_HREF = `/auth/sign-in?next=${encodeURIComponent('/settings')}`

/** Adds, renames and removes the signed-in person's passkeys; they sit beside the password, never replace it. */
export function PasskeysSection() {
  const { data: passkeys, error: loadError, mutate } = usePasskeys()
  const register = useRegisterPasskey()
  const rename = useRenamePasskey()
  const remove = useRemovePasskey()
  const signOut = useSignOut()
  const router = useRouter()

  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [needsReauth, setNeedsReauth] = useState(false)
  const [editing, setEditing] = useState<{ id: string; name: string } | null>(null)
  const [removing, setRemoving] = useState<Passkey | null>(null)

  const run = async (action: () => Promise<unknown>, fallback: string) => {
    setBusy(true)
    setError('')
    setNeedsReauth(false)
    try {
      await action()
      await mutate()
      return true
    } catch (err) {
      if (err instanceof ConnectError && err.code === Code.FailedPrecondition) {
        setNeedsReauth(true)
      } else if (err instanceof ConnectError) {
        setError(err.rawMessage)
      } else if (err instanceof DOMException && err.name === 'NotAllowedError') {
        setError('The passkey prompt was cancelled.')
      } else {
        setError(fallback)
      }
      return false
    } finally {
      setBusy(false)
    }
  }

  const add = async (e: React.FormEvent) => {
    e.preventDefault()
    if (await run(() => register(name.trim()), 'Could not add the passkey.')) setName('')
  }

  const saveName = async () => {
    if (!editing) return
    if (await run(() => rename(editing.id, editing.name), 'Could not rename the passkey.')) {
      setEditing(null)
    }
  }

  const confirmRemove = async () => {
    if (!removing) return
    await run(() => remove(removing.id), 'Could not remove the passkey.')
    setRemoving(null)
  }

  const reauthenticate = async () => {
    await signOut()
    router.replace(REAUTH_HREF)
  }

  return (
    <SettingsSection
      title="Passkeys"
      description="Sign in with your fingerprint, face or a security key. A passkey is an extra method: your password keeps working."
    >
      {error && <Alert tone="danger">{error}</Alert>}
      {loadError && <Alert tone="danger">Could not load your passkeys.</Alert>}
      {needsReauth && (
        <Alert tone="warn">
          <p>For safety, adding or removing a passkey needs a sign-in within the last 5 minutes.</p>
          <Button variant="secondary" size="sm" onClick={() => void reauthenticate()}>
            Sign in again
          </Button>
        </Alert>
      )}

      {passkeys && passkeys.length === 0 && (
        <EmptyState placement="inline">No passkeys yet.</EmptyState>
      )}
      {passkeys && passkeys.length > 0 && (
        <ul aria-label="Your passkeys" className="space-y-2">
          {passkeys.map((p) => (
            <li key={p.id} className="flex flex-wrap items-center gap-2">
              {editing?.id === p.id ? (
                <>
                  <Input
                    aria-label="New passkey name"
                    className="min-w-0 flex-1"
                    maxLength={64}
                    value={editing.name}
                    onChange={(e) => setEditing({ id: p.id, name: e.target.value })}
                  />
                  <Button
                    size="sm"
                    disabled={busy || !editing.name.trim()}
                    onClick={() => void saveName()}
                  >
                    Save
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => setEditing(null)}>
                    Cancel
                  </Button>
                </>
              ) : (
                <>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">{p.name}</p>
                    <p className="text-xs text-muted">
                      Added {formatDate(p.createdAt)}
                      {p.lastUsedAt && ` · Last used ${formatDate(p.lastUsedAt)}`}
                    </p>
                  </div>
                  <Button
                    variant="secondary"
                    size="sm"
                    aria-label={`Rename ${p.name}`}
                    disabled={busy}
                    onClick={() => setEditing({ id: p.id, name: p.name })}
                  >
                    Rename
                  </Button>
                  <Button
                    variant="destructive"
                    size="sm"
                    aria-label={`Remove ${p.name}`}
                    disabled={busy}
                    onClick={() => setRemoving(p)}
                  >
                    Remove
                  </Button>
                </>
              )}
            </li>
          ))}
        </ul>
      )}

      {passkeysSupported() ? (
        <form onSubmit={add} className="space-y-3">
          <Field label="Passkey name" htmlFor="passkey_name">
            <Input
              id="passkey_name"
              placeholder="e.g. Laptop"
              maxLength={64}
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </Field>
          <Button type="submit" disabled={busy || !name.trim()}>
            {busy ? 'Waiting…' : 'Add a passkey'}
          </Button>
        </form>
      ) : (
        <p className="text-sm text-muted">This browser can&apos;t create passkeys.</p>
      )}

      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
        title={`Remove ${removing?.name ?? 'passkey'}?`}
        description="It can no longer be used to sign in. Your password still works."
        confirmLabel="Remove"
        pendingLabel="Removing…"
        destructive
        pending={busy}
        onConfirm={() => void confirmRemove()}
      />
    </SettingsSection>
  )
}
