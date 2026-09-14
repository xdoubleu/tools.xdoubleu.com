'use client'

import { useAutomatedActions } from '@/hooks/useMonitoring'
import AutomatedActionsCard from './AutomatedActionsCard'

export default function ObservabilityClient() {
  const automatedActions = useAutomatedActions()

  return (
    <div className="space-y-4">
      <AutomatedActionsCard data={automatedActions.data} />
    </div>
  )
}
