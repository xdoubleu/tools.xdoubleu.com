import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/cn'

interface FeedItemCategoriesProps {
  categories: string[]
  className?: string
}

/** A feed item's source categories as chips; renders nothing when empty. */
export default function FeedItemCategories({ categories, className }: FeedItemCategoriesProps) {
  if (categories.length === 0) return null
  return (
    <ul aria-label="Categories" className={cn('flex flex-wrap gap-1', className)}>
      {categories.map((category) => (
        <li key={category} className="min-w-0">
          <Badge variant="secondary" className="max-w-full wrap-anywhere">
            {category}
          </Badge>
        </li>
      ))}
    </ul>
  )
}
