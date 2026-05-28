'use client'

import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'

export default function OvertimeWidget() {
  const [minutes, setMinutes] = useState(0)

  useEffect(() => {
    const stored = localStorage.getItem('overtime_minutes')
    if (stored) setMinutes(parseInt(stored, 10))
  }, [])

  const saveMinutes = (newMinutes: number) => {
    setMinutes(newMinutes)
    localStorage.setItem('overtime_minutes', String(newMinutes))
  }

  const hours = Math.floor(Math.abs(minutes) / 60)
  const mins = Math.abs(minutes) % 60
  const sign = minutes < 0 ? '-' : '+'
  const display = `${sign}${hours}h ${mins}m`

  return (
    <div className="mb-4 rounded-2xl border border-border bg-card p-4 shadow-card">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <span className="font-semibold text-lg text-fg">{display}</span>
        <div className="flex flex-wrap gap-2">
          <Button size="sm" onClick={() => saveMinutes(minutes + 60)}>
            +1h
          </Button>
          <Button size="sm" onClick={() => saveMinutes(minutes + 15)}>
            +15m
          </Button>
          <Button size="sm" variant="secondary" onClick={() => saveMinutes(minutes - 15)}>
            -15m
          </Button>
          <Button size="sm" variant="secondary" onClick={() => saveMinutes(minutes - 60)}>
            -1h
          </Button>
          <Button size="sm" variant="ghost" onClick={() => saveMinutes(0)}>
            Reset
          </Button>
        </div>
      </div>
    </div>
  )
}
