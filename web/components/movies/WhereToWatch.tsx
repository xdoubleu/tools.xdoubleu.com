import { Badge } from '@/components/ui/badge'
import { SectionHeading } from '@/components/ui/section-heading'
import { logoUrl } from '@/lib/movies/format'
import type { ProviderOffers } from '@/lib/gen/movies/v1/movies_pb'

const OFFER_LABELS: Record<string, string> = {
  flatrate: 'Stream',
  free: 'Free',
  ads: 'With ads',
  rent: 'Rent',
  buy: 'Buy'
}

interface WhereToWatchProps {
  offers: ProviderOffers[]
  watchLink: string
}

/** Belgian providers by offer type, logos hotlinked from TMDB; data by JustWatch. Marks the user's services. */
export default function WhereToWatch({ offers, watchLink }: WhereToWatchProps) {
  return (
    <section aria-labelledby="where-to-watch" className="space-y-2">
      <SectionHeading level="eyebrow" as="h2" id="where-to-watch">
        Where to watch (BE)
      </SectionHeading>
      {offers.length === 0 ? (
        <p className="text-sm text-muted">Not available to stream in Belgium</p>
      ) : (
        <div className="space-y-3">
          {offers.map((group) => (
            <div key={group.offerType}>
              <SectionHeading level="eyebrow" as="h3" className="mb-1">
                {OFFER_LABELS[group.offerType] ?? group.offerType}
              </SectionHeading>
              <ul className="flex flex-wrap gap-2">
                {group.providers.map((p) => (
                  <li key={p.id} className="flex items-center gap-2 text-sm">
                    {p.logoPath && (
                      <img
                        src={logoUrl(p.logoPath)}
                        alt=""
                        width={32}
                        height={32}
                        className="rounded-lg"
                        loading="lazy"
                      />
                    )}
                    {p.name}
                    {p.mine && <Badge variant="success">Yours</Badge>}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      )}
      <p className="text-xs text-muted">
        Streaming availability data by JustWatch.
        {watchLink && (
          <a
            href={watchLink}
            target="_blank"
            rel="noopener noreferrer"
            className="ml-1 inline-flex min-h-11 items-center underline"
          >
            See all on TMDB
          </a>
        )}
      </p>
    </section>
  )
}
