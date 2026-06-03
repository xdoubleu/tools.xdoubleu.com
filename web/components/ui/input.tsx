import { forwardRef, type InputHTMLAttributes } from 'react'

type InputProps = InputHTMLAttributes<HTMLInputElement>

const Input = forwardRef<HTMLInputElement, InputProps>(({ className = '', ...props }, ref) => {
  return (
    <input
      ref={ref}
      className={[
        'flex h-11 w-full rounded-xl border border-input-border bg-input px-3 py-2',
        'text-sm text-input-text placeholder:text-muted',
        'transition-colors',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:border-accent',
        'disabled:pointer-events-none disabled:opacity-50',
        className
      ]
        .filter(Boolean)
        .join(' ')}
      {...props}
    />
  )
})

Input.displayName = 'Input'

export { Input }
export type { InputProps }
