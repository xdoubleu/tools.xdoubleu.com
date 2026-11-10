import { useState, type FormEvent } from 'react'
import { useFactoryActions, useFactoryAgents, useFactoryTask } from '@/hooks/useFactory'
import { Harness, TaskStatus, type Job } from '@/lib/gen/factory/v1/factory_pb'
import {
  accountsOf,
  actorLabel,
  FIX_CI_REPLY,
  HARNESSES,
  canFixCi,
  jobActive,
  jobLimitNote,
  jobRunner,
  jobStatusLabel,
  prStatusLine,
  priorityLabel,
  shortTaskId,
  statusLabel
} from '@/lib/factory/board'
import { MoveSelect } from '@/components/factory/MoveSelect'
import JobLog from '@/components/factory/JobLog'
import TaskDialog from '@/components/factory/TaskDialog'
import { Alert } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Field } from '@/components/ui/field'
import { FactoryModelPicker } from '@/components/factory/FactoryModelPicker'
import { Select } from '@/components/ui/select'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { Textarea } from '@/components/ui/textarea'

function CommentForm({ taskId }: { taskId: string }) {
  const { comment } = useFactoryActions()
  const [body, setBody] = useState('')
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    setFailed(false)
    try {
      await comment(taskId, body)
      setBody('')
    } catch {
      setFailed(true)
    }
    setPending(false)
  }

  return (
    <form className="space-y-2" onSubmit={(e) => void submit(e)}>
      <Field label="Add a comment" htmlFor="task-comment">
        <Textarea
          id="task-comment"
          rows={3}
          value={body}
          onChange={(e) => setBody(e.target.value)}
        />
      </Field>
      {failed && <Alert tone="danger">Couldn&apos;t post the comment. Try again.</Alert>}
      <Button type="submit" disabled={pending || body.trim() === ''}>
        {pending ? 'Posting…' : 'Comment'}
      </Button>
    </form>
  )
}

/** Reply to the agent: resumes the last job's session on the same branch. */
function ReplyForm({ taskId, latest }: { taskId: string; latest: Job }) {
  const { reply } = useFactoryActions()
  const [text, setText] = useState('')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')

  const send = async (body: string) => {
    setPending(true)
    setError('')
    try {
      await reply(taskId, body)
      setText('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Couldn\u2019t send the reply. Try again.')
    }
    setPending(false)
  }

  const submit = (e: FormEvent) => {
    e.preventDefault()
    void send(text)
  }

  return (
    <form className="space-y-2" aria-label="Reply" onSubmit={submit}>
      <Field label="Reply to the agent" htmlFor="task-reply-text">
        <Textarea
          id="task-reply-text"
          rows={2}
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
      </Field>
      {error && <Alert tone="danger">{error}</Alert>}
      <div className="flex flex-wrap gap-2">
        <Button type="submit" disabled={pending || text.trim() === ''}>
          {pending ? 'Sending…' : 'Reply'}
        </Button>
        {canFixCi(latest) && (
          <Button
            type="button"
            variant="secondary"
            disabled={pending}
            onClick={() => void send(FIX_CI_REPLY)}
          >
            Reply: fix CI
          </Button>
        )}
      </div>
    </form>
  )
}

/** Run the task on the runner, and what its latest job is doing. */
function RunSection({
  taskId,
  latest,
  done
}: {
  taskId: string
  latest: Job | undefined
  done: boolean
}) {
  const { run, stop } = useFactoryActions()
  const [text, setText] = useState('')
  // Harness.UNSPECIFIED is "the repo's default".
  const [harness, setHarness] = useState<Harness>(Harness.UNSPECIFIED)
  const [model, setModel] = useState('')
  const [pending, setPending] = useState(false)
  const [stopping, setStopping] = useState(false)
  const [error, setError] = useState('')
  const active = jobActive(latest)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    setError('')
    try {
      if (harness === Harness.UNSPECIFIED) await run(taskId, text)
      else await run(taskId, text, { harness, model: model.trim() })
      setText('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Couldn\u2019t run the task. Try again.')
    }
    setPending(false)
  }

  const halt = async () => {
    setStopping(true)
    setError('')
    try {
      await stop(taskId)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Couldn\u2019t stop the job. Try again.')
    }
    setStopping(false)
  }

  return (
    <Card variant="inset" padding="md" className="space-y-2" aria-label="Run">
      {latest && (
        <p className="break-words text-xs text-muted" data-testid="job-status">
          Job {jobStatusLabel(latest.status).toLowerCase()} · {latest.branch}
          {jobRunner(latest) && ` · ${jobRunner(latest)}`}
          {jobLimitNote(latest) && ` · ${jobLimitNote(latest)}`}
        </p>
      )}
      {latest?.prUrl && (
        <p className="flex flex-wrap items-center break-words text-xs" data-testid="job-pr">
          <Button asChild variant="link" className="h-auto p-0 text-xs">
            <a href={latest.prUrl} target="_blank" rel="noopener noreferrer">
              Pull request
            </a>
          </Button>
          {prStatusLine(latest) && <span className="text-muted"> · {prStatusLine(latest)}</span>}
        </p>
      )}
      {latest && <JobLog jobId={latest.id} active={active} />}
      {latest && !active && !done && <ReplyForm taskId={taskId} latest={latest} />}
      <form className="space-y-2" onSubmit={(e) => void submit(e)}>
        <Field label="Instructions for this run (optional)" htmlFor="task-run-text">
          <Textarea
            id="task-run-text"
            rows={2}
            value={text}
            onChange={(e) => setText(e.target.value)}
          />
        </Field>
        <Field label="Harness" htmlFor="task-run-harness">
          <Select
            id="task-run-harness"
            value={harness}
            onChange={(e) => setHarness(Number(e.target.value))}
          >
            <option value={Harness.UNSPECIFIED}>Repo default</option>
            {HARNESSES.map((h) => (
              <option key={h.harness} value={h.harness}>
                {h.label}
              </option>
            ))}
          </Select>
        </Field>
        {harness === Harness.OPENCODE && (
          <FactoryModelPicker
            id="task-run-model"
            label="Model"
            hint="An OpenRouter model, such as vendor/model. Pick one or type a custom id; blank uses the repo's default."
            value={model}
            onChange={setModel}
          />
        )}
        {error && <Alert tone="danger">{error}</Alert>}
        <div className="flex flex-wrap gap-2">
          <Button type="submit" disabled={pending || active}>
            {pending ? 'Queuing…' : active ? 'Running…' : 'Run'}
          </Button>
          {active && (
            <Button
              type="button"
              variant="destructive"
              disabled={stopping}
              onClick={() => void halt()}
            >
              {stopping ? 'Stopping…' : 'Stop'}
            </Button>
          )}
        </div>
      </form>
    </Card>
  )
}

/** The short id with a button that copies the full one. */
function TaskId({ id }: { id: string }) {
  const [copied, setCopied] = useState(false)

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(id)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Clipboard access can be denied; the short id stays visible.
    }
  }

  return (
    <div className="flex items-center gap-2 text-xs text-muted">
      <span data-testid="task-id">{shortTaskId(id)}</span>
      <Button variant="secondary" size="sm" onClick={() => void copy()}>
        {copied ? 'Copied' : 'Copy id'}
      </Button>
    </div>
  )
}

interface TaskDetailProps {
  id: string
  onClose: () => void
}

/** A task's description, comments and controls, in a sheet. */
export default function TaskDetail({ id, onClose }: TaskDetailProps) {
  const { data, error, isLoading } = useFactoryTask(id)
  const agents = accountsOf(useFactoryAgents().data)
  const [editing, setEditing] = useState(false)
  const task = data?.task

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent side="sheet" className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{task?.title ?? 'Task'}</DialogTitle>
          <DialogClose />
        </DialogHeader>
        <DialogDescription className="mb-4">
          {task
            ? `${statusLabel(task.status)} · ${actorLabel(task.createdBy, agents)}`
            : 'Task details'}
        </DialogDescription>
        {error && !data && <ErrorState what="the task" />}
        {isLoading && !data && <LoadingState label="task" />}
        {task && data && (
          <div className="space-y-4">
            <TaskId id={task.id} />
            <div className="flex flex-wrap items-center gap-2">
              <Badge>{priorityLabel(task.priority)}</Badge>
              {task.labels.map((l) => (
                <Badge key={l} variant="secondary">
                  {l}
                </Badge>
              ))}
            </div>
            <p className="whitespace-pre-wrap break-words text-sm">
              {task.body || 'No description.'}
            </p>
            <MoveSelect task={task} />
            <Button variant="secondary" onClick={() => setEditing(true)}>
              Edit
            </Button>
            <RunSection
              taskId={task.id}
              latest={data.jobs[0]}
              done={task.status === TaskStatus.DONE}
            />
            <section className="space-y-2" aria-label="Comments">
              {data.comments.length === 0 ? (
                <EmptyState placement="inline">No comments yet.</EmptyState>
              ) : (
                data.comments.map((c) => (
                  <Card key={c.id} variant="inset">
                    <p className="text-xs text-muted">{actorLabel(c.author, agents)}</p>
                    <p className="whitespace-pre-wrap break-words text-sm">{c.body}</p>
                  </Card>
                ))
              )}
            </section>
            <CommentForm taskId={task.id} />
          </div>
        )}
        {editing && task && data && (
          <TaskDialog task={task} body={task.body} onClose={() => setEditing(false)} />
        )}
      </DialogContent>
    </Dialog>
  )
}
