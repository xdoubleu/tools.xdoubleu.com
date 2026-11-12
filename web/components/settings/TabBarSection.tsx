import { useState } from 'react'
import { ArrowDown, ArrowUp } from 'lucide-react'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { SettingsSection } from '@/components/ui/settings-section'
import { useCurrentUser, useSetTabPins } from '@/hooks/useAuth'
import { defaultPins, pinnableApps, resolvePins } from '@/lib/apps'

const MAX_PINS = 3

/** Chooses and orders the apps in the phone tab bar; the desktop sidebar lists every app. */
export function TabBarSection() {
  const { data } = useCurrentUser()
  const setTabPins = useSetTabPins()
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  if (!data) return null

  const pinned = resolvePins(data)
  const others = pinnableApps(data).filter((a) => !pinned.includes(a))

  const save = async (next: string[]) => {
    setError(null)
    setSaving(true)
    try {
      await setTabPins(next)
    } catch {
      setError('Could not save the tab bar. Try again.')
    } finally {
      setSaving(false)
    }
  }
  const ids = pinned.map((a) => a.id)
  const move = (index: number, by: -1 | 1) => {
    const next = [...ids]
    const [item] = next.splice(index, 1)
    next.splice(index + by, 0, item)
    return save(next)
  }

  return (
    <SettingsSection
      title="Tab bar"
      description={`The apps in the bottom bar on your phone, up to ${MAX_PINS}. The sidebar on a wide screen always lists every app.`}
    >
      {error && <Alert tone="danger">{error}</Alert>}
      <ul aria-label="Pinned apps" className="space-y-2">
        {pinned.map((app, i) => (
          <li key={app.id} className="flex items-center gap-2">
            <app.icon aria-hidden className="size-5 shrink-0 text-muted" />
            <span className="min-w-0 flex-1 truncate text-sm">{app.label}</span>
            <Button
              variant="ghost"
              size="iconSm"
              aria-label={`Move ${app.label} up`}
              disabled={saving || i === 0}
              onClick={() => move(i, -1)}
            >
              <ArrowUp aria-hidden className="size-4" />
            </Button>
            <Button
              variant="ghost"
              size="iconSm"
              aria-label={`Move ${app.label} down`}
              disabled={saving || i === pinned.length - 1}
              onClick={() => move(i, 1)}
            >
              <ArrowDown aria-hidden className="size-4" />
            </Button>
            <Button
              variant="secondary"
              size="sm"
              aria-label={`Unpin ${app.label}`}
              disabled={saving}
              onClick={() => save(ids.filter((id) => id !== app.id))}
            >
              Unpin
            </Button>
          </li>
        ))}
      </ul>
      {pinned.length === 0 && <p className="text-sm text-muted">Nothing pinned.</p>}
      {others.length > 0 && (
        <ul aria-label="Other apps" className="space-y-2">
          {others.map((app) => (
            <li key={app.id} className="flex items-center gap-2">
              <app.icon aria-hidden className="size-5 shrink-0 text-muted" />
              <span className="min-w-0 flex-1 truncate text-sm">{app.label}</span>
              <Button
                variant="secondary"
                size="sm"
                aria-label={`Pin ${app.label}`}
                disabled={saving || pinned.length >= MAX_PINS}
                onClick={() => save([...ids, app.id])}
              >
                Pin
              </Button>
            </li>
          ))}
        </ul>
      )}

      {data.tabPinsCustomised && (
        <Button
          variant="secondary"
          size="sm"
          disabled={saving}
          onClick={() => save(defaultPins(data).map((a) => a.id))}
        >
          Reset to most used
        </Button>
      )}
    </SettingsSection>
  )
}
