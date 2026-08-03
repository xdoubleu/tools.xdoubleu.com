'use client'

import Link from 'next/link'
import { useFeedItems } from '@/hooks/useFeeds'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import RssIcon from '@/components/RssIcon'

export default function FeedButton() {
  const { data } = useFeedItems()
  const unreadCount = (data?.items ?? []).filter((item) => !item.readAt && !item.dismissed).length

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
