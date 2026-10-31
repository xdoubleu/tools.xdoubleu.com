import { useState, type FormEvent } from 'react'
import { TaskPriority, type Task } from '@/lib/gen/factory/v1/factory_pb'
import { PRIORITIES, parseLabels, repoLabel, reposOf } from '@/lib/factory/board'
import { useFactoryActions, useFactoryRepos } from '@/hooks/useFactory'
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

interface TaskDialogProps {
  /** Editing this task; absent creates one. */
  task?: Task
  /** The task's body, which the list leaves empty. */
  body?: string
  /** Repo preselected for a new task; the first repo when empty. */
  defaultRepoId?: string
  onClose: () => void
  onSaved?: (id: string) => void
}

/** Create or edit sheet for a task's title, body, priority and labels. */
export default function TaskDialog({
  task,
  body,
  defaultRepoId = '',
  onClose,
  onSaved
}: TaskDialogProps) {
  const { create, update } = useFactoryActions()
  const repos = reposOf(useFactoryRepos().data)
  const [pickedRepoId, setPickedRepoId] = useState(defaultRepoId)
  // The only repo is chosen for the user; with several, they pick one.
  const repoId = pickedRepoId || (repos.length === 1 ? repos[0].id : '')
  const [title, setTitle] = useState(task?.title ?? '')
  const [text, setText] = useState(body ?? '')
  const [priority, setPriority] = useState<TaskPriority>(task?.priority ?? TaskPriority.P2)
  const [labels, setLabels] = useState(task?.labels.join(', ') ?? '')
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    setFailed(false)
    const fields = { title, body: text, priority, labels: parseLabels(labels) }
    try {
      if (task) {
        await update(task.id, fields)
        onSaved?.(task.id)
      } else {
        const created = await create({ ...fields, repoId })
        onSaved?.(created?.id ?? '')
      }
      onClose()
    } catch {
      setFailed(true)
      setPending(false)
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent side="sheet" className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{task ? 'Edit task' : 'New task'}</DialogTitle>
        </DialogHeader>
        <form className="space-y-4" onSubmit={(e) => void submit(e)}>
          {!task && (
            <Field label="Repo" htmlFor="task-repo">
              <Select
                id="task-repo"
                value={repoId}
                onChange={(e) => setPickedRepoId(e.target.value)}
              >
                <option value="" disabled>
                  Choose a repo
                </option>
                {repos.map((r) => (
                  <option key={r.id} value={r.id}>
                    {repoLabel(r)}
                  </option>
                ))}
              </Select>
            </Field>
          )}
          <Field label="Title" htmlFor="task-title">
            <Input
              id="task-title"
              value={title}
              maxLength={200}
              required
              onChange={(e) => setTitle(e.target.value)}
            />
          </Field>
          <Field label="Description" htmlFor="task-body">
            <Textarea
              id="task-body"
              rows={6}
              value={text}
              onChange={(e) => setText(e.target.value)}
            />
          </Field>
          <Field label="Priority" htmlFor="task-priority">
            <Select
              id="task-priority"
              value={priority}
              onChange={(e) => setPriority(Number(e.target.value))}
            >
              {PRIORITIES.map((p) => (
                <option key={p.priority} value={p.priority}>
                  {p.label}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Labels" htmlFor="task-labels" hint="Comma-separated.">
            <Input id="task-labels" value={labels} onChange={(e) => setLabels(e.target.value)} />
          </Field>
          {failed && <Alert tone="danger">Couldn&apos;t save the task. Try again.</Alert>}
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={onClose}>
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={pending || title.trim() === '' || (!task && repoId === '')}
            >
              {pending ? 'Saving…' : 'Save'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
