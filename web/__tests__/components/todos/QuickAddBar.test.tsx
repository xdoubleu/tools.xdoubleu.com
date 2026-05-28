import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import QuickAddBar from '@/components/todos/QuickAddBar'

jest.mock('@/lib/client', () => ({
  createServiceClient: () => ({
    quickAddTask: jest.fn().mockResolvedValue({})
  })
}))

const sections = [
  { id: 's1', name: 'Backlog' },
  { id: 's2', name: 'In Progress' }
]

const labelPresets = [
  { value: 'bug', color: '#ff0000' },
  { value: 'feature', color: '#00ff00' }
]

describe('QuickAddBar', () => {
  it('renders input and submit button', () => {
    render(<QuickAddBar sections={sections} labelPresets={labelPresets} onAdded={jest.fn()} />)
    expect(screen.getByPlaceholderText(/Add task/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add Task' })).toBeInTheDocument()
  })

  it('does not submit when input is empty', () => {
    const onAdded = jest.fn()
    render(<QuickAddBar sections={sections} labelPresets={labelPresets} onAdded={onAdded} />)
    fireEvent.submit(screen.getByRole('button', { name: 'Add Task' }).closest('form')!)
    expect(onAdded).not.toHaveBeenCalled()
  })

  it('calls onAdded and clears input on submit', async () => {
    const onAdded = jest.fn()
    render(<QuickAddBar sections={sections} labelPresets={labelPresets} onAdded={onAdded} />)
    const input = screen.getByPlaceholderText(/Add task/) as HTMLInputElement

    fireEvent.change(input, { target: { value: 'Fix the bug' } })
    fireEvent.submit(input.closest('form')!)

    await waitFor(() => {
      expect(onAdded).toHaveBeenCalled()
      expect(input.value).toBe('')
    })
  })

  it('shows label dropdown when @ is typed', () => {
    render(<QuickAddBar sections={sections} labelPresets={labelPresets} onAdded={jest.fn()} />)
    const input = screen.getByPlaceholderText(/Add task/)
    fireEvent.change(input, { target: { value: '@' } })
    expect(screen.getByText('bug')).toBeInTheDocument()
    expect(screen.getByText('feature')).toBeInTheDocument()
  })

  it('shows section dropdown when # is typed', () => {
    render(<QuickAddBar sections={sections} labelPresets={labelPresets} onAdded={jest.fn()} />)
    const input = screen.getByPlaceholderText(/Add task/)
    fireEvent.change(input, { target: { value: '#' } })
    expect(screen.getByText('Backlog')).toBeInTheDocument()
    expect(screen.getByText('In Progress')).toBeInTheDocument()
  })

  it('selecting a label fills input and hides dropdown', () => {
    render(<QuickAddBar sections={sections} labelPresets={labelPresets} onAdded={jest.fn()} />)
    const input = screen.getByPlaceholderText(/Add task/) as HTMLInputElement
    fireEvent.change(input, { target: { value: 'task @' } })
    fireEvent.click(screen.getByText('bug'))
    expect(input.value).toContain('@bug')
    expect(screen.queryByText('feature')).not.toBeInTheDocument()
  })

  it('selecting a section fills input and hides dropdown', () => {
    render(<QuickAddBar sections={sections} labelPresets={labelPresets} onAdded={jest.fn()} />)
    const input = screen.getByPlaceholderText(/Add task/) as HTMLInputElement
    fireEvent.change(input, { target: { value: 'task #' } })
    fireEvent.click(screen.getByText('Backlog'))
    expect(input.value).toContain('#Backlog')
    expect(screen.queryByText('In Progress')).not.toBeInTheDocument()
  })

  it('hides dropdown when plain text typed', () => {
    render(<QuickAddBar sections={sections} labelPresets={labelPresets} onAdded={jest.fn()} />)
    const input = screen.getByPlaceholderText(/Add task/)
    fireEvent.change(input, { target: { value: '@bug' } })
    fireEvent.change(input, { target: { value: 'hello' } })
    expect(screen.queryByText('bug')).not.toBeInTheDocument()
  })
})
