import { type HTMLAttributes } from 'react'

/**
 * Shared hover/focus treatment for clickable cards (Links or buttons rendered
 * as cards). Apply alongside layout classes (`block`, padding, `cursor-pointer`)
 * so every navigable card elevates the same way. Pairs with `cn()` for overrides.
 * `active:` mirrors `hover:` so touch devices (no hover) still get visible
 * press feedback.
 */
const interactiveCardClass =
  'rounded-2xl border border-border bg-card shadow-card transition-[box-shadow,transform] duration-200 hover:shadow-elevated hover:ring-1 hover:ring-accent/30 active:shadow-elevated active:ring-1 active:ring-accent/30 active:scale-[0.98]'

function Card({ className = '', ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={['rounded-2xl border border-border bg-card shadow-card', className]
        .filter(Boolean)
        .join(' ')}
      {...props}
    />
  )
}

function CardHeader({ className = '', ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={['flex flex-col space-y-1 p-4 sm:p-5', className].filter(Boolean).join(' ')}
      {...props}
    />
  )
}

function CardTitle({ className = '', ...props }: HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h3
      className={['text-base font-semibold text-fg', className].filter(Boolean).join(' ')}
      {...props}
    />
  )
}

function CardDescription({ className = '', ...props }: HTMLAttributes<HTMLParagraphElement>) {
  return <p className={['text-sm text-muted', className].filter(Boolean).join(' ')} {...props} />
}

function CardContent({ className = '', ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={['p-4 pt-0 sm:p-5 sm:pt-0', className].filter(Boolean).join(' ')} {...props} />
  )
}

function CardFooter({ className = '', ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={['flex items-center p-4 pt-0 sm:p-5 sm:pt-0', className].filter(Boolean).join(' ')}
      {...props}
    />
  )
}

export { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter }
export { interactiveCardClass }
