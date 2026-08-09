'use client'

import { useEffect, useState } from 'react'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { getTheme, setTheme, THEMES, type Theme } from '@/lib/theme'

const labels: Record<Theme, string> = {
  auto: 'Auto (match system)',
  light: 'Light',
  dark: 'Dark'
}

export function AppearanceSection() {
  // Read after mount: the stored theme lives in localStorage, unavailable to SSR.
  const [theme, setThemeState] = useState<Theme>('auto')
  useEffect(() => setThemeState(getTheme()), [])

  return (
    <section>
      <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-muted">Appearance</h2>
      <p className="mb-4 text-sm text-subtle">
        Applies to this device only. Auto follows your system light/dark setting.
      </p>
      <RadioGroup name="theme" value={theme} onChange={(value) => setThemeState(setTheme(value))}>
        {THEMES.map((value) => (
          <RadioGroupItem key={value} value={value} label={labels[value]} />
        ))}
      </RadioGroup>
    </section>
  )
}
