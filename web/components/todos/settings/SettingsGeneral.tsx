'use client'

import { useState, useEffect } from 'react'
import { createServiceClient } from '@/lib/client'
import { SettingsService } from '@/lib/gen/todos/v1/settings_pb'
import type { GetSettingsResponse } from '@/lib/gen/todos/v1/settings_pb'

const OBSIDIAN_VAULT_KEY = 'todos:obsidian_vault'

interface Props {
  data: GetSettingsResponse
  mutate: () => void
}

export function SettingsGeneral({ data, mutate }: Props) {
  const hideShortcutHints = data.userSettings?.hideShortcutHints ?? false
  const [vault, setVault] = useState('')

  useEffect(() => {
    setVault(localStorage.getItem(OBSIDIAN_VAULT_KEY) ?? '')
  }, [])

  async function handleToggleHints() {
    const client = createServiceClient(SettingsService)
    await client.updateHideShortcutHints({ hide: !hideShortcutHints })
    mutate()
  }

  function handleVaultChange(e: React.ChangeEvent<HTMLInputElement>) {
    const v = e.target.value
    setVault(v)
    if (v.trim()) {
      localStorage.setItem(OBSIDIAN_VAULT_KEY, v.trim())
    } else {
      localStorage.removeItem(OBSIDIAN_VAULT_KEY)
    }
  }

  return (
    <section aria-labelledby="general-heading">
      <h2
        id="general-heading"
        className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted"
      >
        General
      </h2>
      <div className="space-y-4">
        <label className="flex cursor-pointer items-center gap-3">
          <input
            type="checkbox"
            checked={!hideShortcutHints}
            onChange={handleToggleHints}
            className="h-4 w-4 rounded border-border accent-accent"
          />
          <span className="text-sm text-subtle">Show keyboard shortcut hints</span>
        </label>

        <div>
          <label htmlFor="obsidian-vault" className="mb-1 block text-sm text-subtle">
            Obsidian vault name
          </label>
          <input
            id="obsidian-vault"
            type="text"
            value={vault}
            onChange={handleVaultChange}
            placeholder="my-vault"
            className="w-full rounded border border-input-border bg-input px-3 py-1.5 text-sm text-input-text"
          />
          {vault.trim() && (
            <p className="mt-1 truncate text-xs text-muted">
              {`obsidian://open?vault=${encodeURIComponent(vault.trim())}&file=<label>/<title>`}
            </p>
          )}
        </div>
      </div>
    </section>
  )
}
