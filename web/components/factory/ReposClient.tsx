import { useState } from 'react'
import type { Repo } from '@/lib/gen/factory/v1/factory_pb'
import { harnessLabel, repoLabel } from '@/lib/factory/board'
import { useFactoryRepos } from '@/hooks/useFactory'
import RepoDialog from '@/components/factory/RepoDialog'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { PageContainer } from '@/components/ui/page-container'
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
        breadcrumb={[{ label: 'Factory', href: '/factory' }, { label: 'Repos' }]}
        actions={<Button onClick={() => setEditing(undefined)}>Add repo</Button>}
      />
      {error && !data && <ErrorState what="repos" />}
      {isLoading && !data && <LoadingState label="repos" />}
      {data && data.repos.length === 0 && <EmptyState>No repos yet.</EmptyState>}
      {data && data.repos.length > 0 && (
        <ul className="space-y-2" aria-label="Repos">
          {data.repos.map((repo) => (
            <li key={repo.id}>
              <Card variant="inset" className="flex flex-wrap items-center gap-3">
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
              </Card>
            </li>
          ))}
        </ul>
      )}
      {editing !== null && <RepoDialog repo={editing} onClose={() => setEditing(null)} />}
    </PageContainer>
  )
}
