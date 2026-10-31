import { useState, type FormEvent } from 'react'
import { Code, ConnectError } from '@connectrpc/connect'
import type { CreateAgentAccountResponse } from '@/lib/gen/factory/v1/agent_accounts_pb'
import { useFactoryActions } from '@/hooks/useFactory'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

function failureMessage(err: unknown): string {
  const error = ConnectError.from(err)
  if (error.code === Code.PermissionDenied) return "That code didn't work."
  if (error.code === Code.ResourceExhausted) return 'Too many attempts. Try again later.'
  if (error.code === Code.InvalidArgument) return error.rawMessage
  return "Couldn't create the agent. Try again."
}

/** The credentials of a new agent account; the secret can't be shown again. */
function Credentials({
  created,
  onDone
}: {
  created: CreateAgentAccountResponse
  onDone: () => void
}) {
  const [copied, setCopied] = useState(false)
  const copy = async () => {
    await navigator.clipboard.writeText(created.clientSecret)
    setCopied(true)
  }

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
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={() => void copy()}>
          {copied ? 'Copied' : 'Copy secret'}
        </Button>
        <Button type="button" onClick={onDone}>
          Done
        </Button>
      </DialogFooter>
    </div>
  )
}

/** Create sheet: a name and the step-up code, then the credentials once. */
export default function AgentDialog({ onClose }: { onClose: () => void }) {
  const { createAgent } = useFactoryActions()
  const [name, setName] = useState('')
  const [code, setCode] = useState('')
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState('')
  const [created, setCreated] = useState<CreateAgentAccountResponse | null>(null)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    setFailure('')
    try {
      setCreated(await createAgent(name, code.trim()))
    } catch (err) {
      setFailure(failureMessage(err))
    }
    setPending(false)
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent side="sheet" className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{created ? 'Agent created' : 'New agent'}</DialogTitle>
          <DialogDescription>
            {created
              ? 'Use these credentials in the agent’s headersHelper.'
              : 'An agent works on the board under its own account, never as you.'}
          </DialogDescription>
        </DialogHeader>
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
            <Field
              label="Authenticator code"
              htmlFor="agent-code"
              hint="A code from your authenticator app, or a recovery code."
            >
              <Input
                id="agent-code"
                value={code}
                autoComplete="one-time-code"
                autoCapitalize="none"
                autoCorrect="off"
                required
                onChange={(e) => setCode(e.target.value)}
              />
            </Field>
            {failure && <Alert tone="danger">{failure}</Alert>}
            <DialogFooter>
              <Button type="button" variant="secondary" onClick={onClose}>
                Cancel
              </Button>
              <Button type="submit" disabled={pending || name.trim() === '' || code.trim() === ''}>
                {pending ? 'Creating…' : 'Create'}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  )
}
