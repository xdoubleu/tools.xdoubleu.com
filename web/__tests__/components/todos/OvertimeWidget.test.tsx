import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import OvertimeWidget from '@/components/todos/OvertimeWidget'

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {}

  return {
    getItem: (key: string) => store[key] || null,
    setItem: (key: string, value: string) => {
      store[key] = value.toString()
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      store = {}
    }
  }
})()

Object.defineProperty(global, 'localStorage', {
  value: localStorageMock
})

describe('OvertimeWidget', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('renders with initial value from localStorage', () => {
    localStorage.setItem('overtime_minutes', '120')
    render(<OvertimeWidget />)
    expect(screen.getByText('+2h 0m')).toBeInTheDocument()
  })

  it('renders with zero minutes by default', () => {
    render(<OvertimeWidget />)
    expect(screen.getByText('+0h 0m')).toBeInTheDocument()
  })

  it('adds 1 hour when +1h button is clicked', () => {
    render(<OvertimeWidget />)
    const addHourBtn = screen.getByRole('button', { name: '+1h' })
    fireEvent.click(addHourBtn)
    expect(screen.getByText('+1h 0m')).toBeInTheDocument()
  })

  it('adds 15 minutes when +15m button is clicked', () => {
    render(<OvertimeWidget />)
    const addMinBtn = screen.getByRole('button', { name: '+15m' })
    fireEvent.click(addMinBtn)
    expect(screen.getByText('+0h 15m')).toBeInTheDocument()
  })

  it('subtracts 15 minutes when -15m button is clicked', () => {
    localStorage.setItem('overtime_minutes', '30')
    render(<OvertimeWidget />)
    const subMinBtn = screen.getByRole('button', { name: '-15m' })
    fireEvent.click(subMinBtn)
    expect(screen.getByText('+0h 15m')).toBeInTheDocument()
  })

  it('resets to zero when Reset button is clicked', () => {
    localStorage.setItem('overtime_minutes', '120')
    render(<OvertimeWidget />)
    const resetBtn = screen.getByRole('button', { name: 'Reset' })
    fireEvent.click(resetBtn)
    expect(screen.getByText('+0h 0m')).toBeInTheDocument()
  })

  it('shows negative sign for negative minutes', () => {
    localStorage.setItem('overtime_minutes', '-60')
    render(<OvertimeWidget />)
    expect(screen.getByText('-1h 0m')).toBeInTheDocument()
  })
})
