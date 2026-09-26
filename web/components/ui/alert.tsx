import { type HTMLAttributes } from 'react'
import { cn } from '@/lib/cn'

type AlertTone = 'danger' | 'success' | 'warn' | 'info'

const toneClasses: Record<AlertTone, string> = {
  danger: 'border-danger/30 bg-danger/10 text-danger',
  success: 'border-success/30 bg-success/10 text-success',
  warn: 'border-warn/30 bg-warn/10 text-fg',
  info: 'border-accent/30 bg-accent/10 text-fg'
}

interface AlertProps extends HTMLAttributes<HTMLDivElement> {
  tone?: AlertTone
}

/**
 * Inline message banner. `danger` announces immediately (`role="alert"`);
 * the other tones are polite (`role="status"`).
 */
function Alert({ tone = 'info', className, role, ...props }: AlertProps) {
  return (
    <div
      role={role ?? (tone === 'danger' ? 'alert' : 'status')}
      className={cn(
        'break-words rounded-xl border px-4 py-2 text-sm',
        toneClasses[tone],
        className
      )}
      {...props}
    />
  )
}

export { Alert }
export type { AlertProps, AlertTone }
