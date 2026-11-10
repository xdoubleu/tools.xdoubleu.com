import { useState, type FormEvent } from 'react'
import { Code, ConnectError } from '@connectrpc/connect'
import type { AgentAccount } from '@/lib/gen/factory/v1/agent_accounts_pb'
import { useCopyFeedback } from '@/hooks/useCopyFeedback'
import { useAgentActions } from '@/hooks/useUserManagement'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

export function failureMessage(err: unknown, fallback: string): string {
  const error = ConnectError.from(err)
  if (error.code === Code.Unauthenticated) return 'Your session has expired. Sign in again.'
  if (error.code === Code.PermissionDenied) {
    return "That code didn't work, or no authenticator is set up for your account."
  }
  if (error.code === Code.ResourceExhausted) return 'Too many attempts. Try again later.'
  if (error.code === Code.InvalidArgument) return error.rawMessage
  return fallback
}

/** The credentials of a new or rotated agent secret; it can't be shown again. */
export function Credentials({
  created,
  onDone
}: {
  created: { account?: AgentAccount; clientSecret: string }
  onDone: () => void
}) {
  const { copy, copied, failed: copyFailed } = useCopyFeedback()

  return (
    <div className="space-y-4">
      <Alert tone="warn">
        Copy the secret now, for example into your Keychain. It can&apos;t be shown again.
      </Alert>
      <Field label="Client ID" htmlFor="agent-client-id">
        <Input id="agent-client-id" readOnly value={created.account?.clientId ?? ''} />
      </Field>
      <Field label="Client secret" htmlFor="agent-client-secret">
        <Input id="agent-client-secret" readOnly value={created.clientSecret} />
      </Field>
      {copyFailed && (
        <Alert tone="danger">
          Couldn&apos;t copy. Select the secret above and copy it by hand.
        </Alert>
      )}
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={() => void copy(created.clientSecret)}>
          {copied ? 'Copied' : 'Copy secret'}
        </Button>
        <Button type="button" onClick={onDone}>
          Done
        </Button>
      </DialogFooter>
    </div>
  )
}

/** Create sheet: a name, then the credentials once. */
export default function AgentDialog({ onClose }: { onClose: () => void }) {
  const { create: createAgent } = useAgentActions()
  const [name, setName] = useState('')
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState('')
  const [created, setCreated] = useState<Awaited<ReturnType<typeof createAgent>> | null>(null)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    setFailure('')
    try {
      setCreated(await createAgent(name))
    } catch (err) {
      setFailure(failureMessage(err, "Couldn't create the agent. Try again."))
    }
    setPending(false)
  }

  // Once the secret is shown only Done closes the sheet: it can't be shown again.
  return (
    <Dialog open onOpenChange={(open) => !open && !created && onClose()}>
      <DialogContent side="sheet" className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{created ? 'Agent created' : 'New agent'}</DialogTitle>
          {!created && <DialogClose />}
        </DialogHeader>
        <DialogDescription className="mb-4">
          {created
            ? 'Use these credentials in the agent’s headersHelper.'
            : 'An agent acts under its own account, never as you, and reaches only the apps you grant it.'}
        </DialogDescription>
        {created ? (
          <Credentials created={created} onDone={onClose} />
        ) : (
          <form className="space-y-4" onSubmit={(e) => void submit(e)}>
            <Field label="Name" htmlFor="agent-name">
              <Input
                id="agent-name"
                value={name}
                maxLength={80}
                required
                onChange={(e) => setName(e.target.value)}
              />
            </Field>
            {failure && <Alert tone="danger">{failure}</Alert>}
            <DialogFooter>
              <Button type="button" variant="secondary" onClick={onClose}>
                Cancel
              </Button>
              <Button type="submit" disabled={pending || name.trim() === ''}>
                {pending ? 'Creating…' : 'Create'}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  )
}
