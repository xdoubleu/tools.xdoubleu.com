import { type HTMLAttributes } from 'react'
import { cn } from '@/lib/cn'

/**
 * Hover/focus treatment for clickable cards. The accent ring shows at rest so
 * cards read as interactive on touch; it intensifies on hover/active.
 */
const interactiveCardClass =
  'rounded-2xl border border-border bg-card shadow-card ring-1 ring-accent/20 transition-[box-shadow,transform] duration-200 hover:shadow-elevated hover:ring-accent/40 active:shadow-elevated active:ring-accent/40 active:scale-[0.98]'

const cardVariants = {
  default: 'rounded-2xl border border-border bg-card shadow-card',
  inset: 'rounded-2xl border border-border bg-surface p-3'
} as const

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  /** `inset` is a flat, padded panel nested inside a card or dialog (list rows, notes). */
  variant?: keyof typeof cardVariants
}

/**
 * Static surface for grouped content. Use `LinkCard` when the card navigates,
 * `interactiveCardClass` for other clickable cards.
 */
function Card({ variant = 'default', className, ...props }: CardProps) {
  return <div className={cn(cardVariants[variant], className)} {...props} />
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
export type { CardProps }
