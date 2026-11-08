import { useEffect, useMemo, useRef, type UIEvent } from 'react'
import { useJobLog } from '@/hooks/useFactory'
import { logEntries, type LogEntry } from '@/lib/factory/joblog'
import { Alert } from '@/components/ui/alert'
import { Collapsible } from '@/components/ui/collapsible'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

const TITLE_SUMMARY = 60
/** How far from the end still counts as reading the end, in px. */
const FOLLOW_SLACK = 40

/** A tool call is titled by what it ran, so a closed list can be told apart. */
function disclosureTitle(entry: LogEntry): string {
  if (entry.kind !== 'tool' || entry.body === '') return entry.title
  const line = entry.body.replace(/\n[\s\S]*/, '')
  const short = line.length > TITLE_SUMMARY ? `${line.slice(0, TITLE_SUMMARY)}…` : line
  return `${entry.title} · ${short}`
}

function Entry({ entry }: { entry: LogEntry }) {
  switch (entry.kind) {
    case 'text':
      return <p className="whitespace-pre-wrap break-words text-sm">{entry.body}</p>
    case 'result':
    case 'error':
      return (
        <Alert tone={entry.kind === 'error' ? 'danger' : 'success'}>
          <strong>{entry.title}</strong>
          {entry.body && <span className="whitespace-pre-wrap"> · {entry.body}</span>}
        </Alert>
      )
    default:
      // Tool calls and their output stay closed: they are long and mostly noise.
      return (
        <Collapsible title={disclosureTitle(entry)} triggerClassName="text-sm break-all">
          <p className="whitespace-pre-wrap break-all font-mono text-xs text-muted">
            {entry.body || '(empty)'}
          </p>
        </Collapsible>
      )
  }
}

/** What the agent has said and done in a job, following it while it runs. */
export default function JobLog({ jobId, active }: { jobId: string; active: boolean }) {
  const { data, error, isLoading } = useJobLog(jobId, active)
  const entries = useMemo(() => (data ?? []).flatMap(logEntries), [data])
  const box = useRef<HTMLDivElement>(null)
  // Whether the reader is at the end; one who scrolled up to read is left alone.
  const following = useRef(true)

  // Follow the end of a running job's log.
  useEffect(() => {
    if (active && following.current && box.current) {
      box.current.scrollTop = box.current.scrollHeight
    }
  }, [active, entries.length])

  const onScroll = (e: UIEvent<HTMLDivElement>) => {
    const el = e.currentTarget
    following.current = el.scrollHeight - el.scrollTop - el.clientHeight < FOLLOW_SLACK
  }

  if (error && !data) return <ErrorState what="the log" />
  if (isLoading && !data) return <LoadingState label="log" />
  if (entries.length === 0) {
    return <EmptyState placement="inline">{active ? 'No output yet.' : 'No log kept.'}</EmptyState>
  }
  return (
    <div
      ref={box}
      role="log"
      aria-label="Job log"
      onScroll={onScroll}
      className="max-h-80 space-y-2 overflow-y-auto"
    >
      {entries.map((entry) => (
        <Entry key={entry.key} entry={entry} />
      ))}
    </div>
  )
}
