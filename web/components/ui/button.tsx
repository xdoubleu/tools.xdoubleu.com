import { forwardRef, type ButtonHTMLAttributes } from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cn } from '@/lib/cn'

type Variant = 'default' | 'secondary' | 'ghost' | 'destructive' | 'link'
type Size = 'sm' | 'md' | 'lg' | 'icon' | 'iconSm'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  /** Render the child element as the button (e.g. a Next `<Link>`), inheriting button styles. */
  asChild?: boolean
}

const variantClasses: Record<Variant, string> = {
  default: 'bg-accent text-white hover:bg-accent-hover focus-visible:ring-accent/50 shadow-sm',
  secondary: 'bg-surface border border-border text-fg hover:bg-hover focus-visible:ring-border',
  ghost: 'text-fg hover:bg-hover focus-visible:ring-border',
  destructive: 'bg-danger text-white hover:opacity-90 focus-visible:ring-danger/50 shadow-sm',
  link: 'text-accent underline-offset-4 hover:underline focus-visible:ring-accent/50'
}

const sizeClasses: Record<Size, string> = {
  sm: 'h-8 px-3 text-xs rounded-xl',
  md: 'h-11 px-4 text-sm rounded-xl',
  lg: 'h-12 px-6 text-base rounded-xl',
  icon: 'h-11 w-11 rounded-xl',
  iconSm: 'h-6 w-6 text-sm rounded-lg'
}

const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'default', size = 'md', asChild = false, className, ...props }, ref) => {
    const Comp = asChild ? Slot : 'button'
    return (
      <Comp
        ref={ref}
        className={cn(
          'inline-flex items-center justify-center font-medium transition-colors cursor-pointer',
          'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-1',
          'disabled:pointer-events-none disabled:opacity-50',
          variantClasses[variant],
          sizeClasses[size],
          className
        )}
        {...props}
      />
    )
  }
)

Button.displayName = 'Button'

export { Button }
export type { ButtonProps }
