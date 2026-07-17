// Fixed item categories, mirroring the backend enum (models.Category*).
// Every catalog item has exactly one; 'book' is the default and renders
// without a badge.

import type { BadgeProps } from '@/components/ui/badge'

const CATEGORIES = ['book', 'paper', 'article', 'rss'] as const
export type Category = (typeof CATEGORIES)[number]

export const CATEGORY_LABELS: Record<Category, string> = {
  book: 'Book',
  paper: 'Paper',
  article: 'Article',
  rss: 'RSS'
}

/** Badge variant per category; book never renders a badge. */
export const CATEGORY_BADGE_VARIANTS: Record<Category, BadgeProps['variant']> = {
  book: 'secondary',
  paper: 'default',
  article: 'success',
  rss: 'warn'
}

/** Normalizes a proto Book.category (may be empty on older data). */
export function categoryOf(category: string | undefined): Category {
  const match = CATEGORIES.find((c) => c === category)
  return match ?? 'book'
}

export function categoryLabel(category: string | undefined): string {
  return CATEGORY_LABELS[categoryOf(category)]
}
