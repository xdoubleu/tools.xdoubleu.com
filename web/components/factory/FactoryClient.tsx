import { useState } from 'react'
import { groupByStatus } from '@/lib/factory/board'
import { useFactoryTasks } from '@/hooks/useFactory'
import TaskCard from '@/components/factory/TaskCard'
import TaskDetail from '@/components/factory/TaskDetail'
import TaskDialog from '@/components/factory/TaskDialog'
import { Button } from '@/components/ui/button'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

/** The task board: one column per swipe on a phone, side by side on a wide screen. */
export default function FactoryClient() {
  const { data, error, isLoading } = useFactoryTasks()
  const [openId, setOpenId] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)

  return (
    <PageContainer>
      <PageHeader
        title="Factory"
        actions={<Button onClick={() => setCreating(true)}>New task</Button>}
      />
      {error && !data && <ErrorState what="the board" />}
      {isLoading && !data && <LoadingState label="board" />}
      {data && (
        <div className="-mx-4 flex snap-x snap-mandatory gap-3 overflow-x-auto px-4 pb-2 xl:mx-0 xl:grid xl:grid-cols-6 xl:overflow-visible xl:px-0">
          {groupByStatus(data.tasks).map((column) => (
            <section
              key={column.status}
              aria-label={column.label}
              className="w-4/5 shrink-0 snap-center space-y-2 sm:w-1/2 xl:w-auto"
            >
              <h2 className="text-sm font-semibold">
                {column.label} <span className="text-muted">{column.tasks.length}</span>
              </h2>
              {column.tasks.length === 0 ? (
                <EmptyState compact>Nothing here.</EmptyState>
              ) : (
                column.tasks.map((task) => (
                  <TaskCard key={task.id} task={task} onOpen={setOpenId} />
                ))
              )}
            </section>
          ))}
        </div>
      )}
      {openId && <TaskDetail id={openId} onClose={() => setOpenId(null)} />}
      {creating && <TaskDialog onClose={() => setCreating(false)} onSaved={setOpenId} />}
    </PageContainer>
  )
}
