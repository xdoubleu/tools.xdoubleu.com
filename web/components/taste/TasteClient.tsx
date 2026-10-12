'use client'

import TasteEvents from '@/components/taste/TasteEvents'
import TasteStats from '@/components/taste/TasteStats'
import { SectionCard } from '@/components/ui/section-card'

/** Cross-domain stats above the consumption log. */
export default function TasteClient() {
  return (
    <div className="space-y-6">
      <TasteStats />
      <SectionCard title="Log">
        <TasteEvents />
      </SectionCard>
    </div>
  )
}
