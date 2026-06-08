import { Fragment } from 'react'
import Link from 'next/link'
import { cn } from '@/lib/cn'

export interface BreadcrumbItem {
  label: string
  href?: string
}

/**
 * Hierarchical navigation trail. The last item is rendered as the current page
 * (no link). Earlier items render as links when given an `href`. Use this in
 * place of one-off "back" links so navigation looks consistent across apps.
 */
export function Breadcrumb({ items, className }: { items: BreadcrumbItem[]; className?: string }) {
  return (
    <nav
      aria-label="Breadcrumb"
      className={cn('flex flex-wrap items-center gap-2 text-sm text-muted', className)}
    >
      {items.map((item, index) => {
        const isLast = index === items.length - 1
        return (
          <Fragment key={index}>
            {index > 0 && <span aria-hidden="true">/</span>}
            {item.href && !isLast ? (
              <Link href={item.href} className="hover:text-accent">
                {item.label}
              </Link>
            ) : (
              <span className={cn(isLast && 'text-fg')} aria-current={isLast ? 'page' : undefined}>
                {item.label}
              </span>
            )}
          </Fragment>
        )
      })}
    </nav>
  )
}
