import { type HTMLAttributes } from 'react'
import { cn } from '@/lib/cn'

/**
 * Shared hover/focus treatment for clickable cards (Links or buttons rendered
 * as cards). Apply alongside layout classes (`block`, padding, `cursor-pointer`)
 * so every navigable card elevates the same way. Pairs with `cn()` for overrides.
 * The accent ring is visible at rest (not just on hover/press) so clickable
 * cards read as interactive immediately, including on touch devices with no
 * hover state; it intensifies on `hover:`/`active:` for feedback.
 */
const interactiveCardClass =
  'rounded-2xl border border-border bg-card shadow-card ring-1 ring-accent/20 transition-[box-shadow,transform] duration-200 hover:shadow-elevated hover:ring-accent/40 active:shadow-elevated active:ring-accent/40 active:scale-[0.98]'

/** Static surface for grouped content. Use `interactiveCardClass` when the card is clickable. */
function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn('rounded-2xl border border-border bg-card shadow-card', className)}
      {...props}
    />
  )
}

/** Title/description block at the top of a `Card`. */
function CardHeader({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('flex flex-col space-y-1 p-4 sm:p-5', className)} {...props} />
}

/** Heading inside a `CardHeader`. */
function CardTitle({ className, ...props }: HTMLAttributes<HTMLHeadingElement>) {
  return <h3 className={cn('text-base font-semibold text-fg', className)} {...props} />
}

/** Muted supporting line inside a `CardHeader`. */
function CardDescription({ className, ...props }: HTMLAttributes<HTMLParagraphElement>) {
  return <p className={cn('text-sm text-muted', className)} {...props} />
}

/** Main body of a `Card`, padded to line up with `CardHeader`. */
function CardContent({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('p-4 pt-0 sm:p-5 sm:pt-0', className)} {...props} />
}

/** Action row at the bottom of a `Card`. */
function CardFooter({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('flex items-center p-4 pt-0 sm:p-5 sm:pt-0', className)} {...props} />
}

export { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter }
export { interactiveCardClass }
