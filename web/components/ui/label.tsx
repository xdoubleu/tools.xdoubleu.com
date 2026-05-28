import { forwardRef, type LabelHTMLAttributes } from 'react'

type LabelProps = LabelHTMLAttributes<HTMLLabelElement>

const Label = forwardRef<HTMLLabelElement, LabelProps>(({ className = '', ...props }, ref) => {
  return (
    <label
      ref={ref}
      className={[
        'text-sm font-medium text-fg leading-none',
        'peer-disabled:cursor-not-allowed peer-disabled:opacity-70',
        className
      ]
        .filter(Boolean)
        .join(' ')}
      {...props}
    />
  )
})

Label.displayName = 'Label'

export { Label }
export type { LabelProps }
