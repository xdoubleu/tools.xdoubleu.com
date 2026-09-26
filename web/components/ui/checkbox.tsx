import { forwardRef, type InputHTMLAttributes, type ReactNode } from 'react'
import { cn } from '@/lib/cn'

interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  /** Rendered in a 44px-tall wrapping `<label>`; wrap text in `sr-only` to hide it visually. */
  label?: ReactNode
  labelClassName?: string
}

/** Styled native checkbox; pass `label` to get the wrapping `<label>` (a bare one is a 16px target). */
const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(
  ({ label, labelClassName, className, id, ...props }, ref) => {
    const inputEl = (
      <input
        ref={ref}
        id={id}
        type="checkbox"
        className={cn(
          'h-4 w-4 rounded-lg border border-border bg-surface text-accent',
          'cursor-pointer transition-colors',
          'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:ring-offset-1',
          'disabled:pointer-events-none disabled:opacity-50',
          'checked:bg-accent checked:border-accent',
          className
        )}
        {...props}
      />
    )

    if (!label) return inputEl

    return (
      <label
        htmlFor={id}
        className={cn(
          'inline-flex min-h-11 min-w-11 items-center gap-2 cursor-pointer select-none',
          labelClassName
        )}
      >
        {inputEl}
        {typeof label === 'string' ? <span className="text-sm">{label}</span> : label}
      </label>
    )
  }
)

Checkbox.displayName = 'Checkbox'

export { Checkbox }
export type { CheckboxProps }
