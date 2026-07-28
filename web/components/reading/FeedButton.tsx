'use client'

import Link from 'next/link'
import { useLibrary } from '@/hooks/useBooks'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import RssIcon from '@/components/RssIcon'

export default function FeedButton() {
  const { data: libraryData } = useLibrary()
  const unreadCount = (libraryData?.library?.rss ?? []).filter((ub) => ub.status !== 'read').length

  return (
    <Button asChild variant="ghost" size="sm" className="gap-2">
      <Link href="/feeds">
        <RssIcon />
        Feed
        {unreadCount > 0 && <Badge variant="default">{unreadCount}</Badge>}
      </Link>
    </Button>
  )
}
