'use client'

import { Card } from '@/components/ui/card'

interface Tile {
  label: string
  value: string
  tone?: 'default' | 'danger'
}

export default function StatTiles({ tiles }: { tiles: Tile[] }) {
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
      {tiles.map((t) => (
        <Card key={t.label} className="p-4">
          <p className="text-xs font-medium uppercase tracking-wide text-muted">{t.label}</p>
          <p
            className={`mt-1 text-xl font-semibold ${
              t.tone === 'danger' ? 'text-danger' : 'text-fg'
            }`}
          >
            {t.value}
          </p>
        </Card>
      ))}
    </div>
  )
}
