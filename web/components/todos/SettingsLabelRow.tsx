'use client'

import { useState } from 'react'

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
    <div className="flex items-center gap-3 p-3 rounded border border-border">
      <input
        type="color"
        value={selectedColor}
        onChange={(e) => handleColorChange(e.target.value)}
        className="w-8 h-8 rounded cursor-pointer"
        title="Select color"
      />
      <span className="flex-1 font-medium">{value}</span>
      <button
        onClick={() => onRemove(value)}
        className="px-3 py-1 bg-red-600 text-white text-sm rounded hover:bg-red-700"
      >
        Remove
      </button>
    </div>
  )
}
