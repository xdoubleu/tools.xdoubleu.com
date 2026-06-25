import { forwardRef, type InputHTMLAttributes } from 'react'
import { cn } from '@/lib/cn'

interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label?: string
}

// Checkbox wraps a native <input type="checkbox"> with consistent styling.
// Native checkboxes are sanctioned raw elements per web/CLAUDE.md.
const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(
  ({ label, className, id, ...props }, ref) => {
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
      <label htmlFor={id} className="inline-flex items-center gap-2 cursor-pointer select-none">
        {inputEl}
        <span className="text-sm">{label}</span>
      </label>
    )
  }
)

Checkbox.displayName = 'Checkbox'

export { Checkbox }
export type { CheckboxProps }
