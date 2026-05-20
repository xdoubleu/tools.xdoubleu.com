import Link from 'next/link'

export interface AppLink {
  name: string
  label: string
  href: string
  description: string
}

interface AppGridProps {
  apps: AppLink[]
}

export default function AppGrid({ apps }: AppGridProps) {
  if (apps.length === 0) return null

  return (
    <div className="grid gap-4 sm:grid-cols-2">
      {apps.map((app) => (
        <Link
          key={app.name}
          href={app.href}
          className="block rounded-lg border border-border bg-card p-4 hover:bg-accent transition-colors"
        >
          <div className="font-semibold text-fg">{app.label}</div>
          <div className="mt-1 text-sm text-muted">{app.description}</div>
        </Link>
      ))}
    </div>
  )
}
