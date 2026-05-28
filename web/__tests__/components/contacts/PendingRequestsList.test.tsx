import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import PendingRequestsList from '@/components/contacts/PendingRequestsList'
import { ContactSchema } from '@/lib/gen/contacts/v1/contacts_pb'

const pendingContact = create(ContactSchema, {
  id: 'p1',
  contactUserId: 'waiting@example.com',
  displayName: 'Charlie',
  status: 'pending'
})

const confirmedContact = create(ContactSchema, {
  id: 'p2',
  contactUserId: 'other@example.com',
  status: 'confirmed'
})

describe('PendingRequestsList', () => {
  it('shows empty state when no pending contacts', () => {
    render(<PendingRequestsList contacts={[confirmedContact]} />)
    expect(screen.getByText('No pending requests.')).toBeInTheDocument()
  })

  it('renders pending contacts', () => {
    render(<PendingRequestsList contacts={[pendingContact]} />)
    expect(screen.getByText('Charlie')).toBeInTheDocument()
    expect(screen.getAllByText('waiting@example.com').length).toBeGreaterThan(0)
    expect(screen.getByText('Pending')).toBeInTheDocument()
  })

  it('falls back to contactUserId when displayName is empty', () => {
    const contact = { ...pendingContact, displayName: '' }
    render(<PendingRequestsList contacts={[contact]} />)
    expect(screen.getAllByText('waiting@example.com').length).toBeGreaterThan(0)
  })

  it('renders multiple pending contacts', () => {
    const secondContact = create(ContactSchema, {
      id: 'p3',
      contactUserId: 'b@example.com',
      displayName: 'Dave',
      status: 'pending'
    })
    render(<PendingRequestsList contacts={[pendingContact, secondContact]} />)
    expect(screen.getAllByText('Pending')).toHaveLength(2)
  })
})
