'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'

interface SettingsLabelRowProps {
  value: string
  color: string
  onColorChange: (value: string, color: string) => void
  onRemove: (value: string) => void
}

export default function SettingsLabelRow({
  value,
  color,
  onColorChange,
  onRemove
}: SettingsLabelRowProps) {
  const [selectedColor, setSelectedColor] = useState(color)

  const handleColorChange = (newColor: string) => {
    setSelectedColor(newColor)
    onColorChange(value, newColor)
  }

  return (
    <div className="flex items-center gap-3 rounded-xl border border-border bg-card p-3 shadow-card">
      <input
        type="color"
        value={selectedColor}
        onChange={(e) => handleColorChange(e.target.value)}
        className="h-9 w-9 cursor-pointer rounded-lg border-0 bg-transparent p-0"
        title="Select color"
      />
      <span className="flex-1 font-medium text-fg">{value}</span>
      <Button variant="destructive" size="sm" onClick={() => onRemove(value)}>
        Remove
      </Button>
    </div>
  )
}
