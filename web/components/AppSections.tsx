import { SegmentedTabs } from '@/components/ui/segmented-tabs'
import { useRouter, usePathname } from '@/hooks/navigation'
import { APPS } from '@/lib/apps'

interface AppSectionsProps {
  /** `AppEntry.id` of the app whose sections to show. */
  app: string
  /** Rewrites a section href, e.g. to keep the Mine/Family scope. */
  hrefFor?: (href: string) => string
}

/** The one switch between an app's pages (Library / Stats), under its page title; a section can link out to another service. */
export default function AppSections({ app, hrefFor = (href) => href }: AppSectionsProps) {
  const router = useRouter()
  const pathname = usePathname()
  const sections = APPS.find((a) => a.id === app)?.sections ?? []
  const external = new Set(sections.filter((s) => s.external).map((s) => s.href))
  const current = sections
    .filter((s) => pathname === s.href || pathname.startsWith(`${s.href}/`))
    .sort((a, b) => b.href.length - a.href.length)[0]

  return (
    <SegmentedTabs
      aria-label={`${APPS.find((a) => a.id === app)?.label ?? app} sections`}
      className="mb-4"
      value={current?.href ?? null}
      onChange={(href) => {
        if (href === current?.href) return
        // Served by another container, so a full navigation rather than the SPA router.
        if (external.has(href)) window.open(href, '_self')
        else router.push(hrefFor(href))
      }}
      options={sections.map((s) => ({ value: s.href, label: s.label }))}
    />
  )
}
