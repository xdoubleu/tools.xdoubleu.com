import { useEffect, useState, type DragEvent } from 'react'
import { useSearchParams } from '@/hooks/navigation'
import {
  activeRunner,
  awaitingPickupApproval,
  distinctLabels,
  filterByLabel,
  filterByRepo,
  groupByStatus,
  limitNotice,
  repoLabel,
  reposOf,
  runnersOf,
  runnerState,
  TASK_DATA_TYPE
} from '@/lib/factory/board'
import {
  useFactoryActions,
  useFactoryRepos,
  useFactoryRunners,
  useFactoryTasks
} from '@/hooks/useFactory'
import type { Task } from '@/lib/gen/factory/v1/factory_pb'
import TaskCard from '@/components/factory/TaskCard'
import TaskDetail from '@/components/factory/TaskDetail'
import TaskDialog from '@/components/factory/TaskDialog'
import { Code, ConnectError } from '@connectrpc/connect'
import { Alert } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderLink } from '@/components/ui/page-header'
import { Field } from '@/components/ui/field'
import { Select } from '@/components/ui/select'
import { cn } from '@/lib/cn'

const LIMIT_TICK_MS = 30_000

/** The task board: one column per swipe on a phone, side by side on a wide screen. */
export default function FactoryClient() {
  const { data, error, isLoading } = useFactoryTasks()
  const repos = reposOf(useFactoryRepos().data)
  const runner = runnerState(activeRunner(runnersOf(useFactoryRunners().data)), Date.now())
  const [repoFilter, setRepoFilter] = useState('')
  const [labelFilter, setLabelFilter] = useState('')
  // A link such as /factory?task=<id> (from a Slack message) opens that task.
  const [openId, setOpenId] = useState<string | null>(useSearchParams().get('task'))
  const [creating, setCreating] = useState(false)
  const { setPickupPaused, move } = useFactoryActions()
  const [pickupError, setPickupError] = useState('')
  const [dragError, setDragError] = useState('')
  const [draggingId, setDraggingId] = useState<string | null>(null)
  const [dragOverStatus, setDragOverStatus] = useState<number | null>(null)
  const paused = data?.pickupPaused === true
  // Ticks, so the notice goes away by itself when the limit lifts.
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), LIMIT_TICK_MS)
    return () => clearInterval(id)
  }, [])
  const limit = limitNotice(data?.claudeLimitedUntil ?? '', now)

  const togglePickup = async () => {
    setPickupError('')
    try {
      await setPickupPaused(!paused)
    } catch (err) {
      setPickupError(
        err instanceof Error ? err.message : 'Couldn\u2019t change auto pickup. Try again.'
      )
    }
  }

  const visible = data ? filterByLabel(filterByRepo(data.tasks, repoFilter), labelFilter) : []
  const labels = distinctLabels(data?.tasks ?? [])

  const dropTask = async (task: Task, status: number) => {
    setDragError('')
    if (task.status === status) return
    try {
      await move(task.id, status)
    } catch (err) {
      setDragError(
        ConnectError.from(err).code === Code.PermissionDenied
          ? 'Only you, in the browser, can mark a task Ready.'
          : 'Couldn\u2019t move the task. Try again.'
      )
    }
  }

  const onDrop = (status: number) => (event: DragEvent<HTMLElement>) => {
    event.preventDefault()
    setDragOverStatus(null)
    const id = event.dataTransfer.getData(TASK_DATA_TYPE)
    const task = data?.tasks.find((t) => t.id === id)
    setDraggingId(null)
    if (task) void dropTask(task, status)
  }

  return (
    <PageContainer className="flex h-dvh flex-col lg:h-[calc(100dvh-9rem)]">
      <PageHeader
        title="Factory"
        actions={
          <>
            <PageHeaderLink href="/factory/runners">Runner</PageHeaderLink>
            <PageHeaderLink href="/factory/agents">Agents</PageHeaderLink>
            <PageHeaderLink href="/factory/repos">Repos</PageHeaderLink>
            <Button onClick={() => setCreating(true)}>New task</Button>
          </>
        }
      />
      <p className="mb-4 flex flex-wrap items-center gap-2 text-sm text-muted">
        Runner <Badge variant={runner.online ? 'success' : 'secondary'}>{runner.label}</Badge>
      </p>
      {limit && (
        <p className="mb-4 flex flex-wrap items-center gap-2 text-sm text-muted">
          Claude <Badge variant="warn">{limit}</Badge>
        </p>
      )}
      {data && repos.some((r) => r.autoPickup) && (
        <div className="mb-4 flex flex-wrap items-center gap-2 text-sm text-muted">
          Auto pickup{' '}
          <Badge variant={paused ? 'warn' : 'success'}>{paused ? 'Paused' : 'On'}</Badge>
          <Button variant="secondary" onClick={() => void togglePickup()}>
            {paused ? 'Resume' : 'Pause'}
          </Button>
        </div>
      )}
      {pickupError && (
        <Alert tone="danger" className="mb-4">
          {pickupError}
        </Alert>
      )}
      <div className="mb-4 flex flex-wrap gap-4">
        {repos.length > 1 && (
          <Field label="Repo" htmlFor="factory-repo-filter" className="min-w-40">
            <Select
              id="factory-repo-filter"
              value={repoFilter}
              onChange={(e) => setRepoFilter(e.target.value)}
            >
              <option value="">All repos</option>
              {repos.map((r) => (
                <option key={r.id} value={r.id}>
                  {repoLabel(r)}
                </option>
              ))}
            </Select>
          </Field>
        )}
        {labels.length > 0 && (
          <Field label="Label" htmlFor="factory-label-filter" className="min-w-40">
            <Select
              id="factory-label-filter"
              value={labelFilter}
              onChange={(e) => setLabelFilter(e.target.value)}
            >
              <option value="">All labels</option>
              {labels.map((l) => (
                <option key={l} value={l}>
                  {l}
                </option>
              ))}
            </Select>
          </Field>
        )}
      </div>
      {dragError && (
        <Alert tone="danger" className="mb-4">
          {dragError}
        </Alert>
      )}
      {error && !data && <ErrorState what="the board" />}
      {isLoading && !data && <LoadingState label="board" />}
      {data && (
        <div className="-mx-4 flex min-h-0 flex-1 snap-x snap-mandatory gap-3 overflow-x-auto px-4 pb-2 2xl:mx-0 2xl:grid 2xl:grid-cols-7 2xl:overflow-visible 2xl:px-0">
          {groupByStatus(visible).map((column) => (
            <section
              key={column.status}
              aria-label={column.label}
              className={cn(
                'flex w-4/5 min-h-0 shrink-0 snap-center flex-col space-y-2 rounded-2xl sm:w-1/2 2xl:w-auto',
                dragOverStatus === column.status && 'ring-2 ring-accent/60'
              )}
              onDragOver={(e) => {
                e.preventDefault()
                setDragOverStatus(column.status)
              }}
              onDragLeave={() => setDragOverStatus(null)}
              onDrop={onDrop(column.status)}
            >
              <h2 className="text-sm font-semibold">
                {column.label} <span className="text-muted">{column.tasks.length}</span>
              </h2>
              <div className="min-h-0 flex-1 space-y-2 overflow-y-auto">
                {column.tasks.length === 0 ? (
                  <EmptyState compact>Nothing here.</EmptyState>
                ) : (
                  column.tasks.map((task) => (
                    <TaskCard
                      key={task.id}
                      task={task}
                      repo={repos.length > 1 ? repos.find((r) => r.id === task.repoId) : undefined}
                      awaitingApproval={awaitingPickupApproval(task, repos)}
                      dragging={draggingId === task.id}
                      onOpen={setOpenId}
                      onDragStart={(e, t) => {
                        e.dataTransfer.setData(TASK_DATA_TYPE, t.id)
                        e.dataTransfer.effectAllowed = 'move'
                        setDraggingId(t.id)
                      }}
                      onDragEnd={() => {
                        setDraggingId(null)
                        setDragOverStatus(null)
                      }}
                    />
                  ))
                )}
              </div>
            </section>
          ))}
        </div>
      )}
      {openId && <TaskDetail id={openId} onClose={() => setOpenId(null)} />}
      {creating && (
        <TaskDialog
          defaultRepoId={repoFilter}
          onClose={() => setCreating(false)}
          onSaved={setOpenId}
        />
      )}
    </PageContainer>
  )
}
