import { forwardRef, type ButtonHTMLAttributes } from 'react'
import { cn } from '@/lib/cn'

type MenuItemProps = ButtonHTMLAttributes<HTMLButtonElement>

const MenuItem = forwardRef<HTMLButtonElement, MenuItemProps>(
  ({ className, type = 'button', ...props }, ref) => {
    return (
      <button
        ref={ref}
        type={type}
        className={cn(
          'flex w-full items-center gap-2 rounded-lg px-4 py-2 text-left text-sm text-fg',
          'transition-colors hover:bg-hover',
          'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent',
          'disabled:pointer-events-none disabled:opacity-50',
          className
        )}
        {...props}
      />
    )
  }
)

MenuItem.displayName = 'MenuItem'

export { MenuItem }
export type { MenuItemProps }
