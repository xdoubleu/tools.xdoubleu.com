import { type ReactNode } from 'react'
import { cn } from '@/lib/cn'

interface FieldErrorProps {
  /** Point the control's `aria-describedby` at this. */
  id?: string
  /** Nothing renders while empty, so it can sit beside a field permanently. */
  children?: ReactNode
  className?: string
}

/** Small announced error line under a form control. Use `Alert tone="danger"` for a form-level error. */
function FieldError({ id, children, className }: FieldErrorProps) {
  if (children === undefined || children === null || children === false || children === '')
    return null
  return (
    <p id={id} role="alert" className={cn('text-xs text-danger', className)}>
      {children}
    </p>
  )
}

export { FieldError }
export type { FieldErrorProps }
