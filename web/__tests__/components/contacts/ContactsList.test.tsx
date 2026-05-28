import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ContactsList from '@/components/contacts/ContactsList'
import { ContactSchema } from '@/lib/gen/contacts/v1/contacts_pb'

const mockDeleteContact = jest.fn()

jest.mock('@/hooks/useContacts', () => ({
  useDeleteContact: () => mockDeleteContact
}))

const confirmedContact = create(ContactSchema, {
  id: 'c1',
  contactUserId: 'user@example.com',
  displayName: 'Alice',
  status: 'confirmed'
})

const pendingContact = create(ContactSchema, {
  id: 'c2',
  contactUserId: 'other@example.com',
  status: 'pending'
})

describe('ContactsList', () => {
  beforeEach(() => {
    mockDeleteContact.mockReset()
    jest.spyOn(window, 'confirm').mockReturnValue(true)
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('shows empty state when no confirmed contacts', () => {
    render(<ContactsList contacts={[pendingContact]} />)
    expect(screen.getByText('No contacts yet.')).toBeInTheDocument()
  })

  it('renders confirmed contacts', () => {
    render(<ContactsList contacts={[confirmedContact]} />)
    expect(screen.getByText('Alice')).toBeInTheDocument()
    expect(screen.getByText('user@example.com')).toBeInTheDocument()
  })

  it('falls back to contactUserId when displayName is empty', () => {
    const contact = { ...confirmedContact, displayName: '' }
    render(<ContactsList contacts={[contact]} />)
    expect(screen.getAllByText('user@example.com').length).toBeGreaterThan(0)
  })

  it('calls deleteContact and onUpdated when confirmed', async () => {
    const onUpdated = jest.fn()
    mockDeleteContact.mockResolvedValue(undefined)
    render(<ContactsList contacts={[confirmedContact]} onUpdated={onUpdated} />)

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))

    await waitFor(() => {
      expect(mockDeleteContact).toHaveBeenCalledWith('c1')
      expect(onUpdated).toHaveBeenCalled()
    })
  })

  it('does not delete when user cancels confirm dialog', async () => {
    jest.spyOn(window, 'confirm').mockReturnValue(false)
    render(<ContactsList contacts={[confirmedContact]} />)

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))

    await waitFor(() => {
      expect(mockDeleteContact).not.toHaveBeenCalled()
    })
  })
})
