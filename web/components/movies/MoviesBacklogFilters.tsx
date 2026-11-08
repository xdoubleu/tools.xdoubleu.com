import { type BacklogFilter, NEW_SEASONS, STATUSES, STATUS_LABELS } from '@/lib/movies/format'
import { Select } from '@/components/ui/select'
import { TogglePill } from '@/components/ui/toggle-pill'

// "New seasons" narrows Watched, so it sits right after it.
const STATUS_OPTIONS = [
  { value: '', label: 'All statuses' },
  ...STATUSES.flatMap((s) => [
    { value: s, label: STATUS_LABELS[s] },
    ...(s === 'watched' ? [{ value: NEW_SEASONS, label: 'New seasons' }] : [])
  ])
]

const TYPE_OPTIONS = [
  { value: '', label: 'All types' },
  { value: 'movie', label: 'Movies' },
  { value: 'series', label: 'Series' }
]

const SORT_OPTIONS = [
  { value: 'added', label: 'Recently added' },
  { value: 'title', label: 'Title' },
  { value: 'release', label: 'Release date' },
  { value: 'rating', label: 'Rating' }
]

interface MoviesBacklogFiltersProps {
  filter: BacklogFilter
  onChange: (filter: BacklogFilter) => void
}

/** Type, status, services and sort controls for the backlog, for a `Toolbar`. */
export default function MoviesBacklogFilters({ filter, onChange }: MoviesBacklogFiltersProps) {
  const set = (patch: Partial<BacklogFilter>) => onChange({ ...filter, ...patch })
  const select = (
    label: string,
    key: 'mediaType' | 'status' | 'sort',
    options: { value: string; label: string }[]
  ) => (
    <Select
      aria-label={label}
      className="w-auto"
      value={filter[key]}
      onChange={(e) => set({ [key]: e.target.value })}
    >
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </Select>
  )

  return (
    <>
      {select('Type', 'mediaType', TYPE_OPTIONS)}
      {select('Status', 'status', STATUS_OPTIONS)}
      <TogglePill
        label="On my services"
        active={filter.onMyServices}
        onClick={() => set({ onMyServices: !filter.onMyServices })}
      />
      {select('Sort', 'sort', SORT_OPTIONS)}
    </>
  )
}
