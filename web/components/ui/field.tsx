import { type ReactNode } from 'react'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/cn'

interface FieldProps {
  label: ReactNode
  /** The control's `id`, so tapping the label focuses it. */
  htmlFor: string
  /** Muted help text under the control. */
  hint?: ReactNode
  /** Validation message; replaces `hint` while set. */
  error?: ReactNode
  children: ReactNode
  className?: string
}

/** A form control with its label and hint/error, stacked. */
function Field({ label, htmlFor, hint, error, children, className }: FieldProps) {
  return (
    <div className={cn('min-w-0 space-y-1.5', className)}>
      <Label htmlFor={htmlFor} className="block text-subtle">
        {label}
      </Label>
      {children}
      {error ? (
        <p className="text-xs text-danger" role="alert">
          {error}
        </p>
      ) : (
        hint && <p className="text-xs text-muted">{hint}</p>
      )}
    </div>
  )
}

export { Field }
export type { FieldProps }
