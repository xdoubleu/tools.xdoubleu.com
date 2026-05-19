'use client'

import { useState, useEffect } from 'react'

export default function OvertimeWidget() {
  const [minutes, setMinutes] = useState(0)

  useEffect(() => {
    const stored = localStorage.getItem('overtime_minutes')
    if (stored) {
      setMinutes(parseInt(stored, 10))
    }
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
    <div className="rounded border border-border bg-card p-4 mb-4">
      <div className="flex items-center justify-between">
        <span className="font-semibold text-lg">{display}</span>
        <div className="flex gap-2">
          <button
            onClick={() => saveMinutes(minutes + 60)}
            className="px-2 py-1 bg-blue-600 text-white text-sm rounded hover:bg-blue-700"
          >
            +1h
          </button>
          <button
            onClick={() => saveMinutes(minutes + 15)}
            className="px-2 py-1 bg-blue-600 text-white text-sm rounded hover:bg-blue-700"
          >
            +15m
          </button>
          <button
            onClick={() => saveMinutes(minutes - 15)}
            className="px-2 py-1 bg-orange-600 text-white text-sm rounded hover:bg-orange-700"
          >
            -15m
          </button>
          <button
            onClick={() => saveMinutes(minutes - 60)}
            className="px-2 py-1 bg-orange-600 text-white text-sm rounded hover:bg-orange-700"
          >
            -1h
          </button>
          <button
            onClick={() => saveMinutes(0)}
            className="px-2 py-1 bg-gray-400 text-white text-sm rounded hover:bg-gray-500"
          >
            Reset
          </button>
        </div>
      </div>
    </div>
  )
}
