import { useState } from 'react'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { SettingsSection } from '@/components/ui/settings-section'
import { getTheme, setTheme, THEMES, type Theme } from '@/lib/theme'

const labels: Record<Theme, string> = {
  auto: 'Auto (match system)',
  light: 'Light',
  dark: 'Dark'
}

export function AppearanceSection() {
  const [theme, setThemeState] = useState<Theme>(getTheme)

  return (
    <SettingsSection
      title="Appearance"
      description="Applies to this device only. Auto follows your system light/dark setting."
    >
      <RadioGroup name="theme" value={theme} onChange={(value) => setThemeState(setTheme(value))}>
        {THEMES.map((value) => (
          <RadioGroupItem key={value} value={value} label={labels[value]} />
        ))}
      </RadioGroup>
    </SettingsSection>
  )
}
