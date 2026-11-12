import { useState } from 'react'
import AppGrid from '@/components/AppGrid'
import { Input } from '@/components/ui/input'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { EmptyState, LoadingState } from '@/components/ui/states'
import { useCurrentUser } from '@/hooks/useAuth'
import { visibleSections } from '@/lib/apps'

export default function AppsPage() {
  const { data } = useCurrentUser()
  const [query, setQuery] = useState('')

  if (!data) return <LoadingState />

  const needle = query.trim().toLowerCase()
  const sections = visibleSections(data)
    .map((s) => ({
      ...s,
      apps: s.apps.filter(
        (a) =>
          a.label.toLowerCase().includes(needle) || a.description.toLowerCase().includes(needle)
      )
    }))
    .filter((s) => s.apps.length > 0)

  return (
    <PageContainer>
      <PageHeader title="Apps" description="Everything you can open." />
      <Input
        type="search"
        aria-label="Search apps"
        placeholder="Search apps"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        className="mb-6"
      />
      {sections.length === 0 ? (
        <EmptyState>No apps match your search.</EmptyState>
      ) : (
        <AppGrid sections={sections} />
      )}
    </PageContainer>
  )
}
