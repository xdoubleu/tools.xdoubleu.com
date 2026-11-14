import { useState } from 'react'
import { Code, ConnectError } from '@connectrpc/connect'
import type { Routine } from '@/lib/gen/factory/v1/factory_pb'
import {
  jobActive,
  jobStatusLabel,
  jobStatusVariant,
  ROUTINE_INTERVALS,
  routineIntervalLabel
} from '@/lib/factory/board'
import { formatDateTime } from '@/lib/dates'
import { useFactoryActions, useFactoryRoutines } from '@/hooks/useFactory'
import AppSections from '@/components/AppSections'
import { Alert } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { GroupedList, GroupedListItem } from '@/components/ui/grouped-list'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { Select } from '@/components/ui/select'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

/** The routines the server runs on a schedule: last run, status, and Run / Stop. */
export default function RoutinesClient() {
  const { data, error, isLoading } = useFactoryRoutines()
  const { saveRoutine, runRoutine, stopRoutine } = useFactoryActions()
  const [failure, setFailure] = useState('')

  const act = async (work: () => Promise<void>) => {
    setFailure('')
    try {
      await work()
    } catch (err) {
      const code = ConnectError.from(err).code
      setFailure(
        code === Code.PermissionDenied
          ? 'Only you, in the browser, can change or run a routine.'
          : code === Code.AlreadyExists
            ? 'That routine has nothing to stop, or already has a job queued or running.'
            : "Couldn't do that. Try again."
      )
    }
  }

  const save = (routine: Routine, change: { enabled?: boolean; intervalMinutes?: number }) =>
    act(() =>
      saveRoutine({
        name: routine.name,
        repoId: routine.repoId,
        enabled: routine.enabled,
        intervalMinutes: routine.intervalMinutes,
        ...change
      })
    )

  return (
    <PageContainer size="narrow">
      <PageHeader title="Routines" />
      <AppSections app="factory" />
      {failure && (
        <Alert tone="danger" className="mb-4">
          {failure}
        </Alert>
      )}
      {error && !data && <ErrorState what="routines" />}
      {isLoading && !data && <LoadingState label="routines" />}
      {data && data.routines.length === 0 && <EmptyState>No routines yet.</EmptyState>}
      {data && data.routines.length > 0 && (
        <GroupedList aria-label="Routines">
          {data.routines.map((routine) => (
            <GroupedListItem key={routine.name}>
              <div className="min-w-0 flex-1">
                <p className="break-words font-medium">{routine.name}</p>
                <p className="text-xs text-muted">
                  {routine.lastJob
                    ? `Last run ${formatDateTime(routine.lastJob.createdAt)}`
                    : 'Never run'}
                </p>
              </div>
              {routine.lastJob && (
                <Badge variant={jobStatusVariant(routine.lastJob.status)}>
                  {jobStatusLabel(routine.lastJob.status)}
                </Badge>
              )}
              <Select
                aria-label={`Schedule of ${routine.name}`}
                value={routine.intervalMinutes}
                onChange={(e) => void save(routine, { intervalMinutes: Number(e.target.value) })}
              >
                {!ROUTINE_INTERVALS.some((i) => i.minutes === routine.intervalMinutes) && (
                  <option value={routine.intervalMinutes}>
                    {routineIntervalLabel(routine.intervalMinutes)}
                  </option>
                )}
                {ROUTINE_INTERVALS.map((i) => (
                  <option key={i.minutes} value={i.minutes}>
                    {i.label}
                  </option>
                ))}
              </Select>
              <Button
                variant="secondary"
                onClick={() => void save(routine, { enabled: !routine.enabled })}
              >
                {routine.enabled ? 'Disable' : 'Enable'}
              </Button>
              {jobActive(routine.lastJob) ? (
                <Button
                  variant="destructive"
                  aria-label={`Stop ${routine.name}`}
                  onClick={() => void act(() => stopRoutine(routine.name))}
                >
                  Stop
                </Button>
              ) : (
                <Button
                  aria-label={`Run ${routine.name}`}
                  onClick={() => void act(() => runRoutine(routine.name))}
                >
                  Run
                </Button>
              )}
            </GroupedListItem>
          ))}
        </GroupedList>
      )}
    </PageContainer>
  )
}
