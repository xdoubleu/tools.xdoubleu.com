import { useState, type FormEvent } from 'react'
import { Code, ConnectError } from '@connectrpc/connect'
import { Harness, type Repo } from '@/lib/gen/factory/v1/factory_pb'
import { HARNESSES } from '@/lib/factory/board'
import { useFactoryActions } from '@/hooks/useFactory'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

const DEFAULT_LIMIT_MINUTES = 60
const MIN_LIMIT_MINUTES = 1
const MAX_LIMIT_MINUTES = 1440

function failureMessage(err: unknown): string {
  const code = ConnectError.from(err).code
  if (code === Code.AlreadyExists) return 'That repo is already configured.'
  if (code === Code.InvalidArgument) return ConnectError.from(err).rawMessage
  return "Couldn't save the repo. Try again."
}

interface RepoDialogProps {
  /** Editing this repo; absent adds one. */
  repo?: Repo
  onClose: () => void
}

/** Add or edit sheet for a repo and the settings its jobs run with. */
export default function RepoDialog({ repo, onClose }: RepoDialogProps) {
  const { createRepo, updateRepo } = useFactoryActions()
  const [owner, setOwner] = useState(repo?.owner ?? '')
  const [name, setName] = useState(repo?.name ?? '')
  const [branch, setBranch] = useState(repo?.defaultBranch ?? 'main')
  const [harness, setHarness] = useState<Harness>(repo?.defaultHarness ?? Harness.CLAUDE_CODE)
  const [limit, setLimit] = useState(String(repo?.timeLimitMinutes ?? DEFAULT_LIMIT_MINUTES))
  const [setup, setSetup] = useState(repo?.setupCommands ?? '')
  const [instructions, setInstructions] = useState(repo?.agentInstructions ?? '')
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState('')
  const minutes = Number(limit)
  const limitValid =
    limit.trim() !== '' &&
    Number.isInteger(minutes) &&
    minutes >= MIN_LIMIT_MINUTES &&
    minutes <= MAX_LIMIT_MINUTES

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    setFailure('')
    const fields = {
      owner,
      name,
      defaultBranch: branch,
      defaultHarness: harness,
      setupCommands: setup,
      agentInstructions: instructions,
      timeLimitMinutes: minutes
    }
    try {
      if (repo) await updateRepo(repo.id, fields)
      else await createRepo(fields)
      onClose()
    } catch (err) {
      setFailure(failureMessage(err))
      setPending(false)
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent side="sheet" className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{repo ? 'Edit repo' : 'Add repo'}</DialogTitle>
        </DialogHeader>
        <form className="space-y-4" onSubmit={(e) => void submit(e)}>
          <Field label="Owner" htmlFor="repo-owner">
            <Input
              id="repo-owner"
              value={owner}
              required
              autoCapitalize="none"
              autoCorrect="off"
              onChange={(e) => setOwner(e.target.value)}
            />
          </Field>
          <Field label="Name" htmlFor="repo-name">
            <Input
              id="repo-name"
              value={name}
              required
              autoCapitalize="none"
              autoCorrect="off"
              onChange={(e) => setName(e.target.value)}
            />
          </Field>
          <Field label="Default branch" htmlFor="repo-branch">
            <Input
              id="repo-branch"
              value={branch}
              autoCapitalize="none"
              autoCorrect="off"
              onChange={(e) => setBranch(e.target.value)}
            />
          </Field>
          <Field label="Default harness" htmlFor="repo-harness">
            <Select
              id="repo-harness"
              value={harness}
              onChange={(e) => setHarness(Number(e.target.value))}
            >
              {HARNESSES.map((h) => (
                <option key={h.harness} value={h.harness}>
                  {h.label}
                </option>
              ))}
            </Select>
          </Field>
          <Field
            label="Time limit (minutes)"
            htmlFor="repo-limit"
            hint="1 to 1440."
            error={limitValid ? undefined : 'Enter a whole number from 1 to 1440.'}
          >
            <Input
              id="repo-limit"
              type="number"
              inputMode="numeric"
              min={1}
              max={1440}
              value={limit}
              onChange={(e) => setLimit(e.target.value)}
            />
          </Field>
          <Field
            label="Setup commands"
            htmlFor="repo-setup"
            hint="Run in the sandbox before the agent."
          >
            <Textarea
              id="repo-setup"
              rows={4}
              className="font-mono"
              value={setup}
              onChange={(e) => setSetup(e.target.value)}
            />
          </Field>
          <Field
            label="Agent instructions"
            htmlFor="repo-instructions"
            hint="Added to every prompt for this repo."
          >
            <Textarea
              id="repo-instructions"
              rows={4}
              value={instructions}
              onChange={(e) => setInstructions(e.target.value)}
            />
          </Field>
          {failure && <Alert tone="danger">{failure}</Alert>}
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={onClose}>
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={pending || owner.trim() === '' || name.trim() === '' || !limitValid}
            >
              {pending ? 'Saving…' : 'Save'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
