import { type ReactNode } from 'react'
import { Input } from '@/components/ui/input'
import { TogglePill } from '@/components/ui/toggle-pill'

interface CollectionFacet {
  id: string
  label: string
  active: boolean
  onToggle: () => void
}

interface CollectionToolbarProps {
  /** What the search looks through, e.g. "recipes": names the field "Search recipes". */
  noun: string
  query: string
  onQueryChange: (query: string) => void
  facets?: CollectionFacet[]
  /** Extra controls (selects, a sort) after the facets. */
  children?: ReactNode
  /** Items shown after search and filters. */
  count: number
}

/**
 * Search, facet chips and the result count above a collection. The search filters the list;
 * adding from a catalogue is a separate control with its own label.
 */
function CollectionToolbar({
  noun,
  query,
  onQueryChange,
  facets = [],
  children,
  count
}: CollectionToolbarProps) {
  return (
    <div className="space-y-3">
      <Input
        type="search"
        aria-label={`Search ${noun}`}
        placeholder={`Search ${noun}`}
        value={query}
        onChange={(e) => onQueryChange(e.target.value)}
      />
      <div className="flex flex-wrap items-center gap-2">
        {facets.map((facet) => (
          <TogglePill
            key={facet.id}
            label={facet.label}
            active={facet.active}
            onClick={facet.onToggle}
          />
        ))}
        {children}
        <p className="ml-auto text-sm text-muted" role="status">
          {count} {count === 1 ? 'result' : 'results'}
        </p>
      </div>
    </div>
  )
}

export { CollectionToolbar }
export type { CollectionFacet, CollectionToolbarProps }
