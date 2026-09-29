import { type ReactNode } from 'react'
import Link from 'next/link'
import SettingsIcon from '@/components/SettingsIcon'
import { Breadcrumb, type BreadcrumbItem } from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'

interface PageHeaderProps {
  title: ReactNode
  /** Trail above the title; the last item is the current page. */
  breadcrumb?: BreadcrumbItem[]
  /** Muted line under the title. */
  description?: ReactNode
  /** Page-level controls; they wrap under the title on narrow screens. */
  actions?: ReactNode
  className?: string
}

/** The page's `<h1>` row. Every page uses this instead of a hand-styled heading. */
function PageHeader({ title, breadcrumb, description, actions, className }: PageHeaderProps) {
  return (
    <header className={cn('mb-6 space-y-3', className)}>
      {breadcrumb && <Breadcrumb items={breadcrumb} />}
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1 basis-60">
          <h1 className="break-words text-2xl font-bold leading-tight sm:text-3xl">{title}</h1>
          {description && <p className="mt-1 text-sm text-muted">{description}</p>}
        </div>
        {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
      </div>
    </header>
  )
}

interface PageHeaderLinkProps {
  href: string
  children: ReactNode
  /** Leading icon; mark it `aria-hidden`. */
  icon?: ReactNode
}

/**
 * Navigation to another page in `PageHeader` `actions`, bordered like the
 * `secondary` command buttons beside it.
 */
function PageHeaderLink({ href, children, icon }: PageHeaderLinkProps) {
  return (
    <Button asChild variant="secondary" size="sm" className="gap-2">
      <Link href={href}>
        {icon}
        {children}
      </Link>
    </Button>
  )
}

/** `PageHeaderLink` to an app's settings page, with the gear icon. */
function PageHeaderSettingsLink({ href }: { href: string }) {
  return (
    <PageHeaderLink href={href} icon={<SettingsIcon />}>
      Settings
    </PageHeaderLink>
  )
}

export { PageHeader, PageHeaderLink, PageHeaderSettingsLink }
export type { PageHeaderProps, PageHeaderLinkProps }
