'use client'

import TasteEvents from '@/components/taste/TasteEvents'
import TasteProfile from '@/components/taste/TasteProfile'
import TasteStats from '@/components/taste/TasteStats'
import { SectionCard } from '@/components/ui/section-card'

/** The taste profile, then cross-domain stats above the consumption log. */
export default function TasteClient() {
  return (
    <div className="space-y-6">
      <TasteProfile />
      <TasteStats />
      <SectionCard title="Log">
        <TasteEvents />
      </SectionCard>
    </div>
  )
}
