import { type ReactNode } from 'react'
import { cn } from '@/lib/cn'

type SectionHeadingLevel = 'section' | 'eyebrow'

const levelClasses: Record<SectionHeadingLevel, string> = {
  section: 'text-xl font-semibold leading-tight tracking-tight text-fg',
  eyebrow: 'text-xs font-semibold uppercase tracking-wide text-muted'
}

interface SectionHeadingProps {
  children: ReactNode
  /** `section` is the serif heading under `PageHeader`; `eyebrow` the small uppercase label. */
  level?: SectionHeadingLevel
  /** Heading element; defaults to `h2` for `section`, `h3` for `eyebrow`. */
  as?: 'h2' | 'h3' | 'h4'
  /** Right-aligned controls on the heading row; they wrap under it on narrow screens. */
  actions?: ReactNode
  id?: string
  className?: string
}

/** Heading for a section of a page. Use it instead of a hand-styled `<h2>`/`<h3>`. */
function SectionHeading({
  children,
  level = 'section',
  as,
  actions,
  id,
  className
}: SectionHeadingProps) {
  const Heading = as ?? (level === 'section' ? 'h2' : 'h3')
  const heading = (
    <Heading
      id={id}
      className={cn(levelClasses[level], 'min-w-0 break-words', !actions && className)}
    >
      {children}
    </Heading>
  )
  if (!actions) return heading
  return (
    <div className={cn('flex flex-wrap items-center justify-between gap-2', className)}>
      {heading}
      <div className="flex flex-wrap items-center gap-2">{actions}</div>
    </div>
  )
}

export { SectionHeading }
export type { SectionHeadingProps, SectionHeadingLevel }
