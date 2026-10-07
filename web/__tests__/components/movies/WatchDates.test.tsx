import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'

import WatchDates from '@/components/movies/WatchDates'
import { todayISO } from '@/lib/movies/format'

function setup(dates: string[], pending = false) {
  const handlers = { onAdd: jest.fn(), onEdit: jest.fn(), onRemove: jest.fn() }
  render(<WatchDates idPrefix="w" dates={dates} pending={pending} {...handlers} />)
  return handlers
}

describe('WatchDates', () => {
  it('lists each watch with its day, marking unknown dates', () => {
    setup([new Date(2026, 0, 2, 12).toISOString(), ''])
    expect(screen.getByLabelText('Watch 1 date')).toHaveValue('2026-01-02')
    expect(screen.getByLabelText('Watch 1 date')).toHaveAttribute('id', 'w-0')
    expect(screen.getByLabelText('Watch 2 date')).toHaveValue('')
    expect(screen.getAllByText('Date unknown')).toHaveLength(1)
  })

  it('shows no list without watches', () => {
    setup([])
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })

  it('edits, removes and adds watches', () => {
    const h = setup(['', ''])
    fireEvent.change(screen.getByLabelText('Watch 2 date'), { target: { value: '2020-03-04' } })
    expect(h.onEdit).toHaveBeenCalledWith(1, '2020-03-04')

    fireEvent.click(screen.getAllByRole('button', { name: 'Remove' })[1])
    expect(h.onRemove).toHaveBeenCalledWith(1)

    fireEvent.click(screen.getByRole('button', { name: 'Watched today' }))
    expect(h.onAdd).toHaveBeenCalledWith(todayISO())
    fireEvent.click(screen.getByRole('button', { name: 'Add unknown date' }))
    expect(h.onAdd).toHaveBeenLastCalledWith('')
  })

  it('holds partial years and cleared dates until blur', () => {
    const h = setup([new Date(2026, 0, 2, 12).toISOString()])
    const input = screen.getByLabelText('Watch 1 date')

    fireEvent.change(input, { target: { value: '0202-01-02' } })
    expect(h.onEdit).not.toHaveBeenCalled()
    expect(input).toHaveValue('0202-01-02')
    fireEvent.change(input, { target: { value: '' } })
    expect(h.onEdit).not.toHaveBeenCalled()

    fireEvent.blur(input)
    expect(h.onEdit).toHaveBeenCalledWith(0, '')
    fireEvent.blur(input)
    expect(h.onEdit).toHaveBeenCalledTimes(1)
  })

  it('does not resend a committed date on blur', () => {
    const h = setup([''])
    const input = screen.getByLabelText('Watch 1 date')
    fireEvent.change(input, { target: { value: '2020-03-04' } })
    fireEvent.blur(input)
    expect(h.onEdit).toHaveBeenCalledTimes(1)
  })

  it('sends nothing when blurred unchanged', () => {
    const h = setup([new Date(2026, 0, 2, 12).toISOString()])
    fireEvent.blur(screen.getByLabelText('Watch 1 date'))
    expect(h.onEdit).not.toHaveBeenCalled()
  })

  it('locks its buttons while saving', () => {
    setup([''], true)
    for (const name of ['Remove', 'Watched today', 'Add unknown date']) {
      expect(screen.getByRole('button', { name })).toBeDisabled()
    }
  })

  it('enables its buttons when idle', () => {
    setup([''])
    expect(screen.getByRole('button', { name: 'Remove' })).toBeEnabled()
  })
})
