'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { GetStorageStatsResponse } from '@/lib/gen/observability/v1/observability_pb'
import { formatBytes, formatCount } from '@/lib/observability'

export default function OrphanedStorageCard({ data }: { data?: GetStorageStatsResponse }) {
  const latest = data?.latest
  const orphanCount = latest ? Number(latest.orphanCount) : 0
  const orphanKeys = latest?.orphanKeys ?? []

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle>Orphaned storage</CardTitle>
          <Badge variant="secondary">R2</Badge>
        </div>
        <CardDescription>
          {latest
            ? `${formatCount(latest.orphanCount)} object(s) with no matching book_files row (${formatBytes(latest.orphanSizeBytes)}).`
            : 'No scan recorded yet.'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {!latest || orphanCount === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No orphaned storage objects.</p>
        ) : (
          <>
            <ul className="space-y-2">
              {orphanKeys.map((key) => (
                <li
                  key={key}
                  className="rounded-lg border border-border bg-surface p-3 font-mono text-xs text-fg"
                >
                  {key}
                </li>
              ))}
            </ul>
            {orphanCount > orphanKeys.length && (
              <p className="mt-2 text-xs text-subtle">
                Showing first {orphanKeys.length} of {formatCount(latest.orphanCount)} orphaned
                objects.
              </p>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
