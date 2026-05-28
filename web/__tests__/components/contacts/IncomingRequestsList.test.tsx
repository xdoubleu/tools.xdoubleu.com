import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import IncomingRequestsList from '@/components/contacts/IncomingRequestsList'
import { ContactSchema } from '@/lib/gen/contacts/v1/contacts_pb'

const mockAccept = jest.fn()
const mockDecline = jest.fn()

jest.mock('@/hooks/useContacts', () => ({
  useAcceptContact: () => mockAccept,
  useDeclineContact: () => mockDecline
}))

const incomingContact = create(ContactSchema, {
  id: 'r1',
  contactUserId: 'sender@example.com',
  displayName: 'Bob',
  status: 'incoming'
})

const confirmedContact = create(ContactSchema, {
  id: 'r2',
  contactUserId: 'other@example.com',
  status: 'confirmed'
})

describe('IncomingRequestsList', () => {
  beforeEach(() => {
    mockAccept.mockReset()
    mockDecline.mockReset()
  })

  it('shows empty state when no incoming requests', () => {
    render(<IncomingRequestsList contacts={[confirmedContact]} />)
    expect(screen.getByText('No incoming requests.')).toBeInTheDocument()
  })

  it('renders incoming contacts', () => {
    render(<IncomingRequestsList contacts={[incomingContact]} />)
    expect(screen.getByText('sender@example.com')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Accept' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Decline' })).toBeInTheDocument()
  })

  it('renders display name input pre-filled', () => {
    render(<IncomingRequestsList contacts={[incomingContact]} />)
    const input = screen.getByPlaceholderText('Display name') as HTMLInputElement
    expect(input.value).toBe('Bob')
  })

  it('calls acceptContact with id and typed name', async () => {
    const onUpdated = jest.fn()
    mockAccept.mockResolvedValue(undefined)
    render(<IncomingRequestsList contacts={[incomingContact]} onUpdated={onUpdated} />)

    // Type a custom display name, then accept
    const input = screen.getByPlaceholderText('Display name')
    fireEvent.change(input, { target: { value: 'Bob Smith' } })
    fireEvent.click(screen.getByRole('button', { name: 'Accept' }))

    await waitFor(() => {
      expect(mockAccept).toHaveBeenCalledWith('r1', 'Bob Smith')
      expect(onUpdated).toHaveBeenCalled()
    })
  })

  it('calls declineContact with id', async () => {
    const onUpdated = jest.fn()
    mockDecline.mockResolvedValue(undefined)
    render(<IncomingRequestsList contacts={[incomingContact]} onUpdated={onUpdated} />)

    fireEvent.click(screen.getByRole('button', { name: 'Decline' }))

    await waitFor(() => {
      expect(mockDecline).toHaveBeenCalledWith('r1')
      expect(onUpdated).toHaveBeenCalled()
    })
  })

  it('updates displayName when typed', () => {
    render(<IncomingRequestsList contacts={[incomingContact]} />)
    const input = screen.getByPlaceholderText('Display name') as HTMLInputElement
    fireEvent.change(input, { target: { value: 'New Name' } })
    expect(input.value).toBe('New Name')
  })
})
