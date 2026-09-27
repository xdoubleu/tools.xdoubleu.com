'use client'

import { useId } from 'react'
import { Combobox } from '@/components/ui/combobox'
import { Field } from '@/components/ui/field'
import { useStationSearch } from '@/hooks/useTrains'
import type { Station } from '@/lib/gen/trains/v1/trains_pb'

/** The station's canonical label, e.g. "Brussel-Zuid / Bruxelles-Midi", computed server-side. */
export function stationDisplayName(station: Station): string {
  return station.displayName
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
  const id = useId()
  const { stations } = useStationSearch(query)
  const stopIdByName = new Map(stations.map((s) => [displayName(s), s.stopId]))

  return (
    <Field label={label} htmlFor={id}>
      <Combobox
        id={id}
        value={query}
        onChange={onQueryChange}
        onSelect={(name) => {
          const stopId = stopIdByName.get(name)
          if (stopId) onSelectStation(stopId, name)
        }}
        suggestions={stations.map(displayName)}
        placeholder={placeholder}
        autoFocus={autoFocus}
      />
    </Field>
  )
}
