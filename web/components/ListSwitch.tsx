'use client'

import { usePathname, useRouter } from 'next/navigation'
import { useSharedList } from '@/hooks/useSharedList'
import { listHref } from '@/lib/listScope'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'

const OPTIONS = [
  { value: 'solo', label: 'Mine' },
  { value: 'shared', label: 'Family' }
]

/** Switches a page between the caller's own list and the family's shared one. */
export default function ListSwitch({ className }: { className?: string }) {
  const router = useRouter()
  const pathname = usePathname()
  const shared = useSharedList()
  return (
    <SegmentedTabs
      aria-label="List"
      className={className}
      value={shared ? 'shared' : 'solo'}
      options={OPTIONS}
      onChange={(next) => router.replace(listHref(pathname, next === 'shared'))}
    />
  )
}
