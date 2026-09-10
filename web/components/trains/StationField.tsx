'use client'

import { Combobox } from '@/components/ui/combobox'
import { useStationSearch } from '@/hooks/useTrains'
import type { Station } from '@/lib/gen/trains/v1/trains_pb'

/** Joins the station's distinct nl/fr/en names, e.g. "Brussel-Zuid / Bruxelles-Midi". */
export function stationDisplayName(station: Station): string {
  return Array.from(new Set([station.nameNl, station.nameFr, station.nameEn]))
    .filter((n) => n !== '')
    .join(' / ')
}

const displayName = stationDisplayName

interface StationFieldProps {
  label: string
  query: string
  onQueryChange: (text: string) => void
  onSelectStation: (stopId: string, name: string) => void
  placeholder: string
  autoFocus?: boolean
}

/** Type-ahead station picker over the SearchStations RPC, used for both origin and destination. */
export default function StationField({
  label,
  query,
  onQueryChange,
  onSelectStation,
  placeholder,
  autoFocus
}: StationFieldProps) {
  const { stations } = useStationSearch(query)
  const stopIdByName = new Map(stations.map((s) => [displayName(s), s.stopId]))

  return (
    <div>
      <label className="mb-1 block text-sm font-medium text-subtle">{label}</label>
      <Combobox
        value={query}
        onChange={onQueryChange}
        onSelect={(name) => {
          const stopId = stopIdByName.get(name)
          if (stopId) onSelectStation(stopId, name)
        }}
        suggestions={stations.map(displayName)}
        placeholder={placeholder}
        autoFocus={autoFocus}
        aria-label={label}
      />
    </div>
  )
}
