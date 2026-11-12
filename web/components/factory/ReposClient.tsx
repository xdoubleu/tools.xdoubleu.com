import { useState } from 'react'
import type { Repo } from '@/lib/gen/factory/v1/factory_pb'
import { harnessLabel, repoLabel } from '@/lib/factory/board'
import { useFactoryRepos } from '@/hooks/useFactory'
import RepoDialog from '@/components/factory/RepoDialog'
import { Button } from '@/components/ui/button'
import { GroupedList, GroupedListItem } from '@/components/ui/grouped-list'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { PageContainer } from '@/components/ui/page-container'
import AppSections from '@/components/AppSections'
import { PageHeader } from '@/components/ui/page-header'

/** The repos the factory runs jobs for, with their settings. */
export default function ReposClient() {
  const { data, error, isLoading } = useFactoryRepos()
  // `null` is closed, `undefined` adds a repo, a repo edits it.
  const [editing, setEditing] = useState<Repo | undefined | null>(null)

  return (
    <PageContainer size="narrow">
      <PageHeader
        title="Repos"
        actions={<Button onClick={() => setEditing(undefined)}>Add repo</Button>}
      />
      <AppSections app="factory" />
      {error && !data && <ErrorState what="repos" />}
      {isLoading && !data && <LoadingState label="repos" />}
      {data && data.repos.length === 0 && <EmptyState>No repos yet.</EmptyState>}
      {data && data.repos.length > 0 && (
        <GroupedList aria-label="Repos">
          {data.repos.map((repo) => (
            <GroupedListItem key={repo.id}>
              <div className="min-w-0 flex-1">
                <p className="break-words font-medium">{repoLabel(repo)}</p>
                <p className="text-xs text-muted">
                  {repo.defaultBranch} · {harnessLabel(repo.defaultHarness)} ·{' '}
                  {repo.timeLimitMinutes} min
                </p>
              </div>
              <Button variant="secondary" onClick={() => setEditing(repo)}>
                Edit
              </Button>
            </GroupedListItem>
          ))}
        </GroupedList>
      )}
      {editing !== null && <RepoDialog repo={editing} onClose={() => setEditing(null)} />}
    </PageContainer>
  )
}
