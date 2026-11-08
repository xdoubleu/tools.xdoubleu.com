import { useEffect, useState, type DragEvent } from 'react'
import { usePathname, useRouter, useSearchParams } from '@/hooks/navigation'
import {
  activeRunner,
  awaitingPickupApproval,
  canCloseEpic,
  distinctLabels,
  epicName,
  epicParent,
  epicProgress,
  filterByLabels,
  filterByRepo,
  groupByStatus,
  isEpicLabel,
  isProductLabel,
  limitNotice,
  PRODUCT_LABELS,
  repoLabel,
  reposOf,
  runnersOf,
  runnerState,
  TASK_DATA_TYPE,
  toggleLabel,
  withGroupLabel
} from '@/lib/factory/board'
import {
  useFactoryActions,
  useFactoryRepos,
  useFactoryRunners,
  useFactoryTasks
} from '@/hooks/useFactory'
import { TaskStatus, type Task } from '@/lib/gen/factory/v1/factory_pb'
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
import { SectionHeading } from '@/components/ui/section-heading'
import { Select } from '@/components/ui/select'
import { TogglePill } from '@/components/ui/toggle-pill'
import { Toolbar } from '@/components/ui/toolbar'
import { cn } from '@/lib/cn'

const LIMIT_TICK_MS = 30_000
const REPO_PARAM = 'repo'
const LABEL_PARAM = 'label'

/** The task board: one column per swipe on a phone, side by side on a wide screen. */
export default function FactoryClient() {
  const { data, error, isLoading } = useFactoryTasks()
  const repos = reposOf(useFactoryRepos().data)
  const runner = runnerState(activeRunner(runnersOf(useFactoryRunners().data)), Date.now())
  const searchParams = useSearchParams()
  const router = useRouter()
  const pathname = usePathname()
  // The filter lives in the URL (?repo=<id>&label=a&label=b), so a refresh or a shared link keeps it.
  const [repoFilter, setRepoFilter] = useState(searchParams.get('repo') ?? '')
  const [labelFilter, setLabelFilter] = useState(searchParams.getAll(LABEL_PARAM))
  // A link such as /factory?task=<id> (from a Slack message) opens that task.
  const [openId, setOpenId] = useState<string | null>(searchParams.get('task'))
  const [creating, setCreating] = useState(false)
  const { setPickupPaused, move } = useFactoryActions()
  const [pickupError, setPickupError] = useState('')
  const [moveError, setMoveError] = useState('')
  const [showDone, setShowDone] = useState(false)
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

  const applyFilter = (repo: string, labels: string[]) => {
    setRepoFilter(repo)
    setLabelFilter(labels)
    const next = new URLSearchParams(searchParams)
    next.delete(REPO_PARAM)
    next.delete(LABEL_PARAM)
    if (repo !== '') next.set(REPO_PARAM, repo)
    labels.forEach((l) => next.append(LABEL_PARAM, l))
    const query = next.toString()
    router.replace(query === '' ? pathname : `${pathname}?${query}`)
  }

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

  const visible = data ? filterByLabels(filterByRepo(data.tasks, repoFilter), labelFilter) : []
  const lower = (l: string) => l.trim().toLowerCase()
  const picked = (inGroup: (l: string) => boolean) => labelFilter.find(inGroup) ?? ''
  const isOther = (l: string) => !isProductLabel(l) && !isEpicLabel(l)
  const allLabels = distinctLabels(data?.tasks ?? [])
  // A stale epic from the URL stays selectable, so the select never shows a blank.
  const epics = [
    ...new Set([...allLabels.filter(isEpicLabel), ...labelFilter.filter(isEpicLabel)])
  ].sort()
  const addable = allLabels.filter(
    (l) => isOther(l) && !labelFilter.some((f) => lower(f) === lower(l))
  )

  const allTasks = data?.tasks ?? []
  const epicLabel = picked(isEpicLabel)
  const parent = epicLabel === '' ? undefined : epicParent(epicLabel, allTasks)
  const progress = epicLabel === '' ? undefined : epicProgress(epicLabel, allTasks)
  const closable = (task: Task) => canCloseEpic(task, allTasks)

  const closeEpic = async (task: Task) => {
    setMoveError('')
    try {
      await move(task.id, TaskStatus.DONE)
    } catch {
      setMoveError('Couldn\u2019t close the epic. Try again.')
    }
  }

  const dropTask = async (task: Task, status: number) => {
    setMoveError('')
    if (task.status === status) return
    try {
      await move(task.id, status)
    } catch (err) {
      setMoveError(
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
      <div className="mb-4 flex flex-wrap items-center gap-x-4 gap-y-2 text-sm text-muted">
        <span className="flex items-center gap-2">
          Runner <Badge variant={runner.online ? 'success' : 'secondary'}>{runner.label}</Badge>
        </span>
        {limit && (
          <span className="flex items-center gap-2">
            Claude <Badge variant="warn">{limit}</Badge>
          </span>
        )}
        {data && repos.some((r) => r.autoPickup) && (
          <span className="flex items-center gap-2">
            Auto pickup{' '}
            <Badge variant={paused ? 'warn' : 'success'}>{paused ? 'Paused' : 'On'}</Badge>
            <Button variant="secondary" size="sm" onClick={() => void togglePickup()}>
              {paused ? 'Resume' : 'Pause'}
            </Button>
          </span>
        )}
      </div>
      {pickupError && (
        <Alert tone="danger" className="mb-4">
          {pickupError}
        </Alert>
      )}
      <Toolbar aria-label="Filter tasks">
        <Select
          aria-label="Product"
          value={picked(isProductLabel)}
          onChange={(e) =>
            applyFilter(repoFilter, withGroupLabel(labelFilter, isProductLabel, e.target.value))
          }
          className="w-full sm:w-auto"
        >
          <option value="">All products</option>
          {PRODUCT_LABELS.map((p) => (
            <option key={p} value={p}>
              {p}
            </option>
          ))}
        </Select>
        {repos.length > 1 && (
          <Select
            aria-label="Repo"
            value={repoFilter}
            onChange={(e) => applyFilter(e.target.value, labelFilter)}
            className="w-full sm:w-auto"
          >
            <option value="">All repos</option>
            {repos.map((r) => (
              <option key={r.id} value={r.id}>
                {repoLabel(r)}
              </option>
            ))}
          </Select>
        )}
        {epics.length > 0 && (
          <Select
            aria-label="Epic"
            value={picked(isEpicLabel)}
            onChange={(e) =>
              applyFilter(repoFilter, withGroupLabel(labelFilter, isEpicLabel, e.target.value))
            }
            className="w-full sm:w-auto"
          >
            <option value="">All epics</option>
            {epics.map((l) => (
              <option key={l} value={l}>
                {epicName(l)}
              </option>
            ))}
          </Select>
        )}
        {addable.length > 0 && (
          <Select
            aria-label="Add label filter"
            value=""
            onChange={(e) =>
              e.target.value !== '' &&
              applyFilter(repoFilter, toggleLabel(labelFilter, e.target.value, true))
            }
            className="w-full sm:w-auto"
          >
            <option value="">Add label…</option>
            {addable.map((l) => (
              <option key={l} value={l}>
                {l}
              </option>
            ))}
          </Select>
        )}
        <TogglePill
          active={showDone}
          label={`Done ${allTasks.filter((t) => t.status === TaskStatus.DONE).length}`}
          aria-label="Show done tasks"
          onClick={() => setShowDone(!showDone)}
        />
        {progress && progress.total > 0 && (
          <span className="flex items-center gap-2 text-sm text-muted">
            {progress.done} of {progress.total} done
            {parent && closable(parent) && (
              <Button size="sm" onClick={() => void closeEpic(parent)}>
                Close epic
              </Button>
            )}
          </span>
        )}
        {labelFilter.filter(isOther).map((l) => (
          <TogglePill
            key={l}
            active
            className="max-w-full truncate"
            label={`${l} ×`}
            aria-label={`Remove label filter ${l}`}
            onClick={() => applyFilter(repoFilter, toggleLabel(labelFilter, l, false))}
          />
        ))}
      </Toolbar>
      {moveError && (
        <Alert tone="danger" className="mb-4">
          {moveError}
        </Alert>
      )}
      {error && !data && <ErrorState what="the board" />}
      {isLoading && !data && <LoadingState label="board" />}
      {data && (
        <div
          className={cn(
            'grid min-h-0 flex-1 grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-5',
            showDone ? 'xl:grid-cols-7' : 'xl:grid-cols-6'
          )}
        >
          {groupByStatus(visible, showDone).map((column) => (
            <section
              key={column.status}
              aria-label={column.label}
              className={cn(
                'flex min-h-0 min-w-0 flex-col space-y-2 rounded-2xl',
                dragOverStatus === column.status && 'ring-2 ring-accent/60'
              )}
              onDragOver={(e) => {
                e.preventDefault()
                setDragOverStatus(column.status)
              }}
              onDragLeave={() => setDragOverStatus(null)}
              onDrop={onDrop(column.status)}
            >
              <SectionHeading level="eyebrow" as="h2">
                {column.label} <span className="font-normal">{column.tasks.length}</span>
              </SectionHeading>
              <div className="min-h-0 flex-1 space-y-2 overflow-y-auto">
                {column.tasks.length === 0 ? (
                  <EmptyState placement="inline">Nothing here.</EmptyState>
                ) : (
                  column.tasks.map((task) => (
                    <TaskCard
                      key={task.id}
                      task={task}
                      repo={repos.length > 1 ? repos.find((r) => r.id === task.repoId) : undefined}
                      awaitingApproval={awaitingPickupApproval(task, repos)}
                      dragging={draggingId === task.id}
                      onCloseEpic={closable(task) ? () => void closeEpic(task) : undefined}
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
