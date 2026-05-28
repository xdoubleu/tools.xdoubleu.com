import React from 'react'
import { render, screen } from '@testing-library/react'
import PendingRequestsList from '@/components/contacts/PendingRequestsList'

const pendingContact = {
  id: 'p1',
  contactUserId: 'waiting@example.com',
  displayName: 'Charlie',
  status: 'pending'
}

const confirmedContact = {
  id: 'p2',
  contactUserId: 'other@example.com',
  displayName: '',
  status: 'confirmed'
}

describe('PendingRequestsList', () => {
  it('shows empty state when no pending contacts', () => {
    render(<PendingRequestsList contacts={[confirmedContact as never]} />)
    expect(screen.getByText('No pending requests.')).toBeInTheDocument()
  })

  it('renders pending contacts', () => {
    render(<PendingRequestsList contacts={[pendingContact as never]} />)
    expect(screen.getByText('Charlie')).toBeInTheDocument()
    expect(screen.getAllByText('waiting@example.com').length).toBeGreaterThan(0)
    expect(screen.getByText('Pending')).toBeInTheDocument()
  })

  it('falls back to contactUserId when displayName is empty', () => {
    const contact = { ...pendingContact, displayName: '' }
    render(<PendingRequestsList contacts={[contact as never]} />)
    expect(screen.getAllByText('waiting@example.com').length).toBeGreaterThan(0)
  })

  it('renders multiple pending contacts', () => {
    const second = {
      id: 'p3',
      contactUserId: 'b@example.com',
      displayName: 'Dave',
      status: 'pending'
    }
    render(<PendingRequestsList contacts={[pendingContact, second] as never[]} />)
    expect(screen.getAllByText('Pending')).toHaveLength(2)
  })
})
