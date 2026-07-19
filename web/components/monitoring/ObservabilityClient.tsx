'use client'

import { useState } from 'react'
import { PageContainer } from '@/components/ui/page-container'
import { Select } from '@/components/ui/select'
import {
  useJobStats,
  useUsageStats,
  useStorageStats,
  useDatabaseStats
} from '@/hooks/useMonitoring'
import { formatBytes, formatCount } from '@/lib/observability'
import StatTiles from './StatTiles'
import StorageCard from './StorageCard'
import DatabaseCard from './DatabaseCard'
import JobsCard from './JobsCard'
import UsageCard from './UsageCard'

const WINDOW_OPTIONS = [7, 30, 90]

export default function ObservabilityClient() {
  const [windowDays, setWindowDays] = useState(30)

  const jobStats = useJobStats(windowDays)
  const usageStats = useUsageStats(windowDays)
  const storageStats = useStorageStats()
  const databaseStats = useDatabaseStats()

  const latest = storageStats.data?.latest
  const failingJobs = (jobStats.data?.stats ?? []).filter((s) => Number(s.failedRuns) > 0).length

  const tiles = [
    {
      label: 'R2 storage',
      value: latest ? formatBytes(latest.totalSizeBytes) : '—'
    },
    {
      label: 'Database',
      value: databaseStats.data ? formatBytes(databaseStats.data.totalSizeBytes) : '—'
    },
    {
      label: 'Orphaned',
      value: latest ? formatBytes(latest.orphanSizeBytes) : '—',
      tone: latest && Number(latest.orphanCount) > 0 ? ('danger' as const) : ('default' as const)
    },
    {
      label: 'Jobs failing',
      value: formatCount(failingJobs),
      tone: failingJobs > 0 ? ('danger' as const) : ('default' as const)
    }
  ]

  return (
    <PageContainer className="p-6">
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-3xl font-bold">Observability</h1>
        <Select
          value={String(windowDays)}
          onChange={(e) => setWindowDays(Number(e.target.value))}
          className="h-9 w-auto"
          aria-label="Time window"
        >
          {WINDOW_OPTIONS.map((d) => (
            <option key={d} value={d}>
              Last {d} days
            </option>
          ))}
        </Select>
      </div>

      <StatTiles tiles={tiles} />

      <div className="mt-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
        <StorageCard data={storageStats.data} />
        <DatabaseCard data={databaseStats.data} />
        <JobsCard data={jobStats.data} />
        <UsageCard data={usageStats.data} />
      </div>
    </PageContainer>
  )
}
