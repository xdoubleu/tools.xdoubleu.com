import { useState } from 'react'
import { Code, ConnectError } from '@connectrpc/connect'
import { type Task, type TaskStatus } from '@/lib/gen/factory/v1/factory_pb'
import { COLUMNS } from '@/lib/factory/board'
import { useFactoryActions } from '@/hooks/useFactory'
import { Select } from '@/components/ui/select'

/** Select that moves a task between columns; shows a message when the move is refused. */
export function MoveSelect({ task }: { task: Task }) {
  const { move } = useFactoryActions()
  const [failure, setFailure] = useState('')

  const onChange = async (status: TaskStatus) => {
    setFailure('')
    try {
      await move(task.id, status)
    } catch (err) {
      setFailure(
        ConnectError.from(err).code === Code.PermissionDenied
          ? 'Only you, in the browser, can mark a task Ready.'
          : "Couldn't move the task. Try again."
      )
    }
  }

  return (
    <>
      <Select
        aria-label={`Move ${task.title}`}
        value={task.status}
        onChange={(e) => void onChange(Number(e.target.value))}
      >
        {COLUMNS.map((c) => (
          <option key={c.status} value={c.status}>
            {c.label}
          </option>
        ))}
      </Select>
      {failure && (
        <p className="text-xs text-danger" role="alert">
          {failure}
        </p>
      )}
    </>
  )
}
