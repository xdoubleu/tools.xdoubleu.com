import { useState, type FormEvent } from 'react'
import { Code, ConnectError } from '@connectrpc/connect'
import type { CreateRunnerResponse } from '@/lib/gen/factory/v1/runners_pb'
import { useFactoryActions } from '@/hooks/useFactory'
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

function failureMessage(err: unknown): string {
  const error = ConnectError.from(err)
  if (error.code === Code.Unauthenticated) return 'Your session has expired. Sign in again.'
  if (error.code === Code.PermissionDenied) {
    return "That code didn't work, or no authenticator is set up for your account."
  }
  if (error.code === Code.ResourceExhausted) return 'Too many attempts. Try again later.'
  if (error.code === Code.AlreadyExists) {
    return 'A runner is already active. Revoke it before pairing another.'
  }
  if (error.code === Code.InvalidArgument) return error.rawMessage
  return "Couldn't create the runner. Try again."
}

/** The pairing code of a new runner; it can't be shown again. */
function PairingCode({ created, onDone }: { created: CreateRunnerResponse; onDone: () => void }) {
  return (
    <div className="space-y-4">
      <Alert tone="warn">
        This code works once and expires in 10 minutes. It can&apos;t be shown again.
      </Alert>
      <Field label="Pairing code" htmlFor="runner-pairing-code">
        <Input id="runner-pairing-code" readOnly value={created.pairingCode} />
      </Field>
      <p className="break-words text-sm text-muted">
        On the Mac, run <code>factory-runner pair {created.pairingCode}</code>.
      </p>
      <DialogFooter>
        <Button type="button" onClick={onDone}>
          Done
        </Button>
      </DialogFooter>
    </div>
  )
}

/** Create sheet: a name and the step-up code, then the pairing code once. */
export default function RunnerDialog({ onClose }: { onClose: () => void }) {
  const { createRunner } = useFactoryActions()
  const [name, setName] = useState('')
  const [code, setCode] = useState('')
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState('')
  const [created, setCreated] = useState<CreateRunnerResponse | null>(null)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    setFailure('')
    try {
      setCreated(await createRunner(name, code.trim()))
    } catch (err) {
      setFailure(failureMessage(err))
    }
    setPending(false)
  }

  // Once the pairing code is shown only Done closes the sheet: it can't be shown again.
  return (
    <Dialog open onOpenChange={(open) => !open && !created && onClose()}>
      <DialogContent side="sheet" className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{created ? 'Runner created' : 'Pair a runner'}</DialogTitle>
          {!created && <DialogClose />}
        </DialogHeader>
        <DialogDescription className="mb-4">
          {created
            ? 'Pair the Mac with this code.'
            : 'The runner is the daemon on your Mac that will run jobs.'}
        </DialogDescription>
        {created ? (
          <PairingCode created={created} onDone={onClose} />
        ) : (
          <form className="space-y-4" onSubmit={(e) => void submit(e)}>
            <Field label="Name" htmlFor="runner-name">
              <Input
                id="runner-name"
                value={name}
                maxLength={80}
                required
                onChange={(e) => setName(e.target.value)}
              />
            </Field>
            <Field
              label="Authenticator code"
              htmlFor="runner-code"
              hint="A code from your authenticator app, or a recovery code."
            >
              <Input
                id="runner-code"
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
