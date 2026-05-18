'use client'

import { useEffect, useState } from 'react'
import type { Policy } from '@/lib/gen/todos/v1/settings_pb'
import {
  isPolicyBannerDismissed,
  dismissPolicyBanner,
} from '@/lib/todos/policiesBanner'

interface PoliciesBannerProps {
  policies: Policy[]
}

export function PoliciesBanner({ policies }: PoliciesBannerProps) {
  const [visible, setVisible] = useState(false)

  const policyIds = policies.map((p) => p.id)
  const reappearAfterHours =
    policies.length > 0 ? policies[0].reappearAfterHours : 24

  useEffect(() => {
    if (policies.length === 0) return
    if (!isPolicyBannerDismissed(policyIds, reappearAfterHours)) {
      setVisible(true)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [policies.length])

  function handleDismiss() {
    dismissPolicyBanner(policyIds)
    setVisible(false)
  }

  if (!visible || policies.length === 0) return null

  return (
    <aside
      role="banner"
      aria-label="Policies"
      className="rounded-lg border border-amber-300 bg-amber-50 p-4"
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1">
          <h2 className="mb-2 text-sm font-semibold text-amber-800">
            Active Policies
          </h2>
          <ul className="space-y-1">
            {policies.map((policy) => (
              <li key={policy.id} className="text-sm text-amber-900">
                {policy.text}
              </li>
            ))}
          </ul>
        </div>
        <button
          type="button"
          onClick={handleDismiss}
          aria-label="Dismiss policies banner"
          className="rounded p-1 text-amber-600 hover:bg-amber-100 hover:text-amber-800"
        >
          ✕
        </button>
      </div>
    </aside>
  )
}
