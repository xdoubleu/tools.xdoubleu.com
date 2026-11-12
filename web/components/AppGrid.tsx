import { type AppEntry } from '@/lib/apps'
import { LinkCard } from '@/components/ui/link-card'
import { SectionHeading } from '@/components/ui/section-heading'

export interface AppSection {
  title: string
  apps: AppEntry[]
}

interface AppGridProps {
  apps?: AppEntry[]
  sections?: AppSection[]
}

function AppCards({ apps }: { apps: AppEntry[] }) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {apps.map((app) => (
        <LinkCard key={app.id} href={app.href} external={app.external} linkClassName="p-4">
          <div className="flex items-center gap-2 font-semibold text-fg">
            <app.icon aria-hidden className="size-5 shrink-0 text-accent" />
            {app.label}
          </div>
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
            <SectionHeading level="eyebrow" as="h2" className="mb-3">
              {section.title}
            </SectionHeading>
            <AppCards apps={section.apps} />
          </section>
        ))}
      </div>
    )
  }

  if (!apps || apps.length === 0) return null

  return <AppCards apps={apps} />
}
