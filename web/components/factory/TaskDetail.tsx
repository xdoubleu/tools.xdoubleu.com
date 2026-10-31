import { useState, type FormEvent } from 'react'
import { useFactoryActions, useFactoryTask } from '@/hooks/useFactory'
import { priorityLabel, statusLabel } from '@/lib/factory/board'
import { MoveSelect } from '@/components/factory/TaskCard'
import TaskDialog from '@/components/factory/TaskDialog'
import { Alert } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Field } from '@/components/ui/field'
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

interface TaskDetailProps {
  id: string
  onClose: () => void
}

/** A task's description, comments and controls, in a sheet. */
export default function TaskDetail({ id, onClose }: TaskDetailProps) {
  const { data, error, isLoading } = useFactoryTask(id)
  const [editing, setEditing] = useState(false)
  const task = data?.task

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent side="sheet" className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{task?.title ?? 'Task'}</DialogTitle>
          <DialogDescription>
            {task ? `${statusLabel(task.status)} · ${task.createdBy}` : 'Task details'}
          </DialogDescription>
        </DialogHeader>
        {error && !data && <ErrorState what="the task" />}
        {isLoading && !data && <LoadingState label="task" />}
        {task && data && (
          <div className="space-y-4">
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
            <section className="space-y-2" aria-label="Comments">
              {data.comments.length === 0 ? (
                <EmptyState compact>No comments yet.</EmptyState>
              ) : (
                data.comments.map((c) => (
                  <Card key={c.id} variant="inset">
                    <p className="text-xs text-muted">{c.author}</p>
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
