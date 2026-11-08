import { type ReactNode } from 'react'
import { SectionHeading } from '@/components/ui/section-heading'
import { cn } from '@/lib/cn'

interface SettingsSectionProps {
  title: ReactNode
  /** Muted line under the title. */
  description?: ReactNode
  /** Controls on the title row. */
  actions?: ReactNode
  children: ReactNode
  className?: string
}

/**
 * One group on a settings page. Stack them in `SettingsSections`, which owns
 * the spacing between groups; don't add dividers.
 */
function SettingsSection({
  title,
  description,
  actions,
  children,
  className
}: SettingsSectionProps) {
  return (
    <section className={cn('space-y-3', className)}>
      <div className="space-y-1">
        <SectionHeading actions={actions}>{title}</SectionHeading>
        {description !== undefined && <p className="text-sm text-muted">{description}</p>}
      </div>
      {children}
    </section>
  )
}

/** Vertical stack of `SettingsSection`s with the standard gap between them. */
function SettingsSections({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('space-y-10', className)}>{children}</div>
}

export { SettingsSection, SettingsSections }
export type { SettingsSectionProps }
