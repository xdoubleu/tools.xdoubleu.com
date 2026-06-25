import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
  SortableHeader
} from '@/components/ui/table'

describe('Table primitives', () => {
  it('renders a table with header and body', () => {
    render(
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>Alice</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    )
    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(screen.getByText('Name')).toBeInTheDocument()
    expect(screen.getByText('Alice')).toBeInTheDocument()
  })

  it('renders a columnheader for TableHead', () => {
    render(
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Title</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody />
      </Table>
    )
    expect(screen.getByRole('columnheader', { name: 'Title' })).toBeInTheDocument()
  })
})

describe('SortableHeader', () => {
  it('calls onSort when clicked', () => {
    const onSort = jest.fn()
    render(
      <Table>
        <TableHeader>
          <TableRow>
            <SortableHeader dir={null} onSort={onSort}>
              Title
            </SortableHeader>
          </TableRow>
        </TableHeader>
        <TableBody />
      </Table>
    )
    fireEvent.click(screen.getByRole('button', { name: 'Title' }))
    expect(onSort).toHaveBeenCalledTimes(1)
  })

  it('shows asc indicator when dir is asc', () => {
    render(
      <Table>
        <TableHeader>
          <TableRow>
            <SortableHeader dir="asc" onSort={jest.fn()}>
              Title
            </SortableHeader>
          </TableRow>
        </TableHeader>
        <TableBody />
      </Table>
    )
    expect(screen.getByRole('button').textContent).toContain('^')
  })

  it('shows desc indicator when dir is desc', () => {
    render(
      <Table>
        <TableHeader>
          <TableRow>
            <SortableHeader dir="desc" onSort={jest.fn()}>
              Title
            </SortableHeader>
          </TableRow>
        </TableHeader>
        <TableBody />
      </Table>
    )
    expect(screen.getByRole('button').textContent).toContain('v')
  })

  it('shows no indicator when dir is null', () => {
    render(
      <Table>
        <TableHeader>
          <TableRow>
            <SortableHeader dir={null} onSort={jest.fn()}>
              Title
            </SortableHeader>
          </TableRow>
        </TableHeader>
        <TableBody />
      </Table>
    )
    const btn = screen.getByRole('button')
    expect(btn.textContent).not.toContain('^')
    expect(btn.textContent).not.toContain('v')
  })
})
