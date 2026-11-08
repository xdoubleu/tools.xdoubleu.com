import { forwardRef, type ButtonHTMLAttributes } from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cn } from '@/lib/cn'
import { controlHeightClass, controlSquareClass } from '@/components/ui/control'

type Variant = 'default' | 'secondary' | 'ghost' | 'destructive' | 'link'
type Size = 'sm' | 'md' | 'lg' | 'icon' | 'iconSm'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  /** Render the child element as the button (e.g. a `<Link>`), inheriting button styles. */
  asChild?: boolean
}

const variantClasses: Record<Variant, string> = {
  default:
    'bg-accent text-white hover:bg-accent-hover active:bg-accent-hover focus-visible:ring-accent/50 shadow-card',
  secondary:
    'bg-surface border border-border text-fg hover:bg-hover active:bg-hover focus-visible:ring-border',
  ghost: 'text-fg hover:bg-hover active:bg-hover focus-visible:ring-border',
  destructive:
    'bg-danger text-white hover:opacity-90 active:opacity-90 focus-visible:ring-danger/50 shadow-card',
  link: 'text-accent underline-offset-4 hover:underline active:underline focus-visible:ring-accent/50'
}

// `md` and `icon` sit on the shared control height. `sm` and `iconSm` are 44px
// on phones too and compact from `sm:` up, for dense rows and tables.
const sizeClasses: Record<Size, string> = {
  sm: 'h-11 px-3 text-sm sm:h-8 sm:text-xs rounded-xl',
  md: cn(controlHeightClass, 'px-4 text-sm rounded-xl'),
  lg: 'h-12 px-6 text-base rounded-xl',
  icon: cn(controlSquareClass, 'rounded-xl'),
  iconSm: 'size-11 text-base sm:size-7 sm:text-sm rounded-lg'
}

// Inline text links ignore `size`; they keep a 44px row height on phones only.
const linkSizeClass = 'h-auto min-h-11 px-0 sm:min-h-0'

const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'default', size = 'md', asChild = false, className, ...props }, ref) => {
    const Comp = asChild ? Slot : 'button'
    return (
      <Comp
        ref={ref}
        className={cn(
          'inline-flex items-center justify-center font-medium cursor-pointer',
          'transition-[color,background-color,box-shadow,transform] duration-press ease-spring active:scale-[0.97] motion-reduce:ease-linear motion-reduce:active:scale-100',
          'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-1',
          'disabled:pointer-events-none disabled:opacity-50',
          variantClasses[variant],
          variant === 'link' ? linkSizeClass : sizeClasses[size],
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
