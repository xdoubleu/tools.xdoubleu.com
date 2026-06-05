import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { Select } from '@/components/ui/select'

describe('Select', () => {
  it('renders options and the rounded-xl shape', () => {
    render(
      <Select aria-label="role" defaultValue="user">
        <option value="user">User</option>
        <option value="admin">Admin</option>
      </Select>
    )
    const select = screen.getByLabelText('role')
    expect(select).toHaveClass('rounded-xl', 'bg-input')
    expect(screen.getByRole('option', { name: 'Admin' })).toBeInTheDocument()
  })

  it('fires onChange', () => {
    const onChange = jest.fn()
    render(
      <Select aria-label="role" value="user" onChange={onChange}>
        <option value="user">User</option>
        <option value="admin">Admin</option>
      </Select>
    )
    fireEvent.change(screen.getByLabelText('role'), { target: { value: 'admin' } })
    expect(onChange).toHaveBeenCalled()
  })

  it('lets className override the default width', () => {
    render(
      <Select aria-label="role" className="w-auto">
        <option value="user">User</option>
      </Select>
    )
    const select = screen.getByLabelText('role')
    expect(select).toHaveClass('w-auto')
    expect(select).not.toHaveClass('w-full')
  })
})
