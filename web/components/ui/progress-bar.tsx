import { cn } from '@/lib/cn'

interface ProgressBarProps {
  value: number
  /** Defaults to 100, so `value` reads as a percentage. */
  max?: number
  /** Names the bar for screen readers, e.g. "Reading progress". */
  'aria-label': string
  /** Text read out instead of the bare number, e.g. "12 of 30 chapters". */
  valueText?: string
  /** Fill colour token; defaults to the accent. */
  tone?: 'accent' | 'success' | 'warn' | 'danger'
  className?: string
}

const toneClasses = {
  accent: 'bg-accent',
  success: 'bg-success',
  warn: 'bg-warn',
  danger: 'bg-danger'
} as const

/** Thin horizontal `role="progressbar"` track; `value` is clamped to `0…max`. */
function ProgressBar({
  value,
  max = 100,
  'aria-label': ariaLabel,
  valueText,
  tone = 'accent',
  className
}: ProgressBarProps) {
  const clamped = Math.min(Math.max(value, 0), max)
  const percent = max > 0 ? (clamped / max) * 100 : 0
  return (
    <div
      role="progressbar"
      aria-label={ariaLabel}
      aria-valuemin={0}
      aria-valuemax={max}
      aria-valuenow={clamped}
      aria-valuetext={valueText}
      className={cn('h-2 w-full overflow-hidden rounded-sm bg-surface', className)}
    >
      <div
        className={cn(
          'h-full rounded-sm transition-[width] duration-settle ease-spring motion-reduce:transition-none',
          toneClasses[tone]
        )}
        style={{ width: `${percent}%` }}
      />
    </div>
  )
}

export { ProgressBar }
export type { ProgressBarProps }
