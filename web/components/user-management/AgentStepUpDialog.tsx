import { useState, type FormEvent } from 'react'
import type { AgentAccount } from '@/lib/gen/factory/v1/agent_accounts_pb'
import { Credentials, failureMessage } from '@/components/user-management/AgentDialog'
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

interface Props {
  title: string
  description: string
  submitLabel: string
  /** Runs the action with the code. A returned secret is shown once, then Done closes. */
  run: (code: string) => Promise<{ account?: AgentAccount; clientSecret: string } | void>
  onClose: () => void
}

/** Authenticator-code sheet for an agent action that needs the step-up check. */
export default function AgentStepUpDialog({
  title,
  description,
  submitLabel,
  run,
  onClose
}: Props) {
  const [code, setCode] = useState('')
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState('')
  const [issued, setIssued] = useState<{ account?: AgentAccount; clientSecret: string } | null>(
    null
  )

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    setFailure('')
    try {
      const result = await run(code.trim())
      if (result) setIssued(result)
      else onClose()
    } catch (err) {
      setFailure(failureMessage(err, "That didn't work. Try again."))
    }
    setPending(false)
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !issued && onClose()}>
      <DialogContent side="sheet" className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{issued ? 'New secret' : title}</DialogTitle>
          {!issued && <DialogClose />}
        </DialogHeader>
        <DialogDescription className="mb-4">
          {issued ? 'The old secret keeps working for 24 hours.' : description}
        </DialogDescription>
        {issued ? (
          <Credentials created={issued} onDone={onClose} />
        ) : (
          <form className="space-y-4" onSubmit={(e) => void submit(e)}>
            <Field
              label="Authenticator code"
              htmlFor="agent-step-up-code"
              hint="A code from your authenticator app, or a recovery code."
            >
              <Input
                id="agent-step-up-code"
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
              <Button type="submit" disabled={pending || code.trim() === ''}>
                {pending ? 'Working…' : submitLabel}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  )
}
