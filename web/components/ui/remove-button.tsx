import { forwardRef, type ButtonHTMLAttributes } from 'react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'

interface RemoveButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'> {
  /** Names what goes, e.g. "Remove Flour". */
  'aria-label': string
}

/** Icon-only × that removes a row, chip or attachment; turns danger on hover and focus. */
const RemoveButton = forwardRef<HTMLButtonElement, RemoveButtonProps>(
  ({ className, type = 'button', ...props }, ref) => (
    <Button
      ref={ref}
      type={type}
      variant="ghost"
      size="iconSm"
      className={cn(
        'shrink-0 text-lg leading-none text-muted hover:text-danger focus-visible:text-danger active:text-danger',
        className
      )}
      {...props}
    >
      <span aria-hidden="true">×</span>
    </Button>
  )
)

RemoveButton.displayName = 'RemoveButton'

export { RemoveButton }
export type { RemoveButtonProps }
