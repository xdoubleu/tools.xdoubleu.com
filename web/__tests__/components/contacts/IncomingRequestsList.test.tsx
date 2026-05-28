import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import IncomingRequestsList from '@/components/contacts/IncomingRequestsList'

const mockAccept = jest.fn()
const mockDecline = jest.fn()

jest.mock('@/hooks/useContacts', () => ({
  useAcceptContact: () => mockAccept,
  useDeclineContact: () => mockDecline
}))

const incomingContact = {
  id: 'r1',
  contactUserId: 'sender@example.com',
  displayName: 'Bob',
  status: 'incoming'
}

const confirmedContact = {
  id: 'r2',
  contactUserId: 'other@example.com',
  displayName: '',
  status: 'confirmed'
}

describe('IncomingRequestsList', () => {
  beforeEach(() => {
    mockAccept.mockReset()
    mockDecline.mockReset()
  })

  it('shows empty state when no incoming requests', () => {
    render(<IncomingRequestsList contacts={[confirmedContact as never]} />)
    expect(screen.getByText('No incoming requests.')).toBeInTheDocument()
  })

  it('renders incoming contacts', () => {
    render(<IncomingRequestsList contacts={[incomingContact as never]} />)
    expect(screen.getByText('sender@example.com')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Accept' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Decline' })).toBeInTheDocument()
  })

  it('renders display name input pre-filled', () => {
    render(<IncomingRequestsList contacts={[incomingContact as never]} />)
    const input = screen.getByPlaceholderText('Display name') as HTMLInputElement
    expect(input.value).toBe('Bob')
  })

  it('calls acceptContact with id and typed name', async () => {
    const onUpdated = jest.fn()
    mockAccept.mockResolvedValue(undefined)
    render(<IncomingRequestsList contacts={[incomingContact as never]} onUpdated={onUpdated} />)

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
    render(<IncomingRequestsList contacts={[incomingContact as never]} onUpdated={onUpdated} />)

    fireEvent.click(screen.getByRole('button', { name: 'Decline' }))

    await waitFor(() => {
      expect(mockDecline).toHaveBeenCalledWith('r1')
      expect(onUpdated).toHaveBeenCalled()
    })
  })

  it('updates displayName when typed', () => {
    render(<IncomingRequestsList contacts={[incomingContact as never]} />)
    const input = screen.getByPlaceholderText('Display name') as HTMLInputElement
    fireEvent.change(input, { target: { value: 'New Name' } })
    expect(input.value).toBe('New Name')
  })
})
