import { LinkCard } from '@/components/ui/link-card'

export interface AppLink {
  name: string
  label: string
  href: string
  description: string
  accessKey?: string
  /** Plain anchor instead of `<Link>`, for targets served by another container (e.g. `/grafana`). */
  external?: boolean
}

export interface AppSection {
  title: string
  apps: AppLink[]
}

interface AppGridProps {
  apps?: AppLink[]
  sections?: AppSection[]
}

function AppCards({ apps }: { apps: AppLink[] }) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {apps.map((app) => (
        <LinkCard key={app.name} href={app.href} external={app.external} linkClassName="p-4">
          <div className="font-semibold text-fg">{app.label}</div>
          <div className="mt-1 text-sm text-muted">{app.description}</div>
        </LinkCard>
      ))}
    </div>
  )
}

export default function AppGrid({ apps, sections }: AppGridProps) {
  if (sections) {
    const visibleSections = sections.filter((s) => s.apps.length > 0)
    if (visibleSections.length === 0) return null

    return (
      <div className="space-y-8">
        {visibleSections.map((section) => (
          <section key={section.title}>
            <h2 className="mb-3 text-xs font-semibold uppercase tracking-widest text-muted">
              {section.title}
            </h2>
            <AppCards apps={section.apps} />
          </section>
        ))}
      </div>
    )
  }

  if (!apps || apps.length === 0) return null

  return <AppCards apps={apps} />
}
