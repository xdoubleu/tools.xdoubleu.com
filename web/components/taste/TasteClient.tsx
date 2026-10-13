'use client'

import ListSwitch from '@/components/ListSwitch'
import TasteEvents from '@/components/taste/TasteEvents'
import TasteProfile from '@/components/taste/TasteProfile'
import TasteRecommendations from '@/components/taste/TasteRecommendations'
import TasteStats from '@/components/taste/TasteStats'
import { useSharedList } from '@/hooks/useSharedList'
import { SectionCard } from '@/components/ui/section-card'

/** The taste profile and picks, then cross-domain stats above the consumption log. */
export default function TasteClient() {
  const shared = useSharedList()
  return (
    <div className="space-y-6">
      <ListSwitch />
      {shared && (
        <p className="text-sm text-muted">
          The family&apos;s shared movies &amp; series list, read as one person. Your own taste
          stays separate.
        </p>
      )}
      <TasteProfile />
      <TasteRecommendations />
      <TasteStats />
      <SectionCard title="Log">
        <TasteEvents />
      </SectionCard>
    </div>
  )
}
