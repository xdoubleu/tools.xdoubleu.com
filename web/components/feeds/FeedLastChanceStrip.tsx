import FeedReaderCard from '@/components/feeds/FeedReaderCard'
import { SectionHeading } from '@/components/ui/section-heading'
import type { Item } from '@/lib/gen/feeds/v1/feeds_pb'

interface FeedLastChanceStripProps {
  items: Item[]
  feedTitleById: ReadonlyMap<string, string>
  lastVisit: number
  pendingRead: ReadonlyMap<string, Item>
  onMarkRead: (item: Item) => void
  onSettled: (itemId: string) => void
}

// Unread items about to expire, soonest first, above the regular list.
export default function FeedLastChanceStrip({
  items,
  feedTitleById,
  lastVisit,
  pendingRead,
  onMarkRead,
  onSettled
}: FeedLastChanceStripProps) {
  if (items.length === 0) return null
  return (
    <section aria-label="Last chance" className="mb-6">
      <SectionHeading level="eyebrow" as="h2" className="mb-2">
        Last chance · expiring within 48 hours
      </SectionHeading>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {items.map((item) => (
          <FeedReaderCard
            key={item.id}
            item={item}
            feedTitle={feedTitleById.get(item.feedId)}
            isNew={new Date(item.createdAt).getTime() > lastVisit}
            isRead={Boolean(item.readAt) || pendingRead.has(item.id)}
            onMarkRead={onMarkRead}
            onSettled={onSettled}
          />
        ))}
      </div>
    </section>
  )
}
