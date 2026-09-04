'use client'

import { StatTile, StatTileGrid } from '@/components/ui/stat'

interface Tile {
  label: string
  value: string
  tone?: 'default' | 'warn' | 'danger'
}

export default function StatTiles({ tiles }: { tiles: Tile[] }) {
  return (
    <StatTileGrid>
      {tiles.map((t) => (
        <StatTile key={t.label} label={t.label} value={t.value} tone={t.tone} />
      ))}
    </StatTileGrid>
  )
}
