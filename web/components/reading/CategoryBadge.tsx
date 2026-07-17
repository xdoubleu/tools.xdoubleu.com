import { Badge } from '@/components/ui/badge'
import { CATEGORY_BADGE_VARIANTS, CATEGORY_LABELS, categoryOf } from '@/lib/reading/categories'

// CategoryBadge marks non-book items (papers, articles, RSS posts) in lists
// and detail views. Books — the default — render nothing to keep a pure book
// library visually unchanged.
export default function CategoryBadge({
  category,
  className
}: {
  category: string | undefined
  className?: string
}) {
  const cat = categoryOf(category)
  if (cat === 'book') return null
  return (
    <Badge variant={CATEGORY_BADGE_VARIANTS[cat]} className={className}>
      {CATEGORY_LABELS[cat]}
    </Badge>
  )
}
