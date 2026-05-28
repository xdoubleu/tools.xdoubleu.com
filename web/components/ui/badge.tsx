import { type HTMLAttributes } from 'react'

type BadgeVariant = 'default' | 'secondary' | 'success' | 'warn' | 'danger'

interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: BadgeVariant
}

const variantClasses: Record<BadgeVariant, string> = {
  default: 'bg-accent/10 text-accent border-accent/20',
  secondary: 'bg-surface text-subtle border-border',
  success: 'bg-success/10 text-success border-success/20',
  warn: 'bg-warn/10 text-warn border-warn/20',
  danger: 'bg-danger/10 text-danger border-danger/20'
}

function Badge({ variant = 'default', className = '', ...props }: BadgeProps) {
  return (
    <span
      className={[
        'inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium',
        variantClasses[variant],
        className
      ]
        .filter(Boolean)
        .join(' ')}
      {...props}
    />
  )
}

export { Badge }
export type { BadgeProps }
