import Link from 'next/link'
import { Card, interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { cn } from '@/lib/cn'

export default function GamesStatCard({
  label,
  value,
  href
}: {
  label: string
  value: string | number
  href?: string
}) {
  if (href) {
    return (
      <Link href={href} className={cn(interactiveCardClass, 'relative block p-3')}>
        <CardLinkStatus />
        <p className="text-xs text-muted">{label}</p>
        <p className="text-xl font-bold mt-0.5">
          {value}{' '}
          <span aria-hidden className="text-sm text-muted">
            &rarr;
          </span>
        </p>
      </Link>
    )
  }

  return (
    <Card className="p-3">
      <p className="text-xs text-muted">{label}</p>
      <p className="text-xl font-bold mt-0.5">{value}</p>
    </Card>
  )
}
