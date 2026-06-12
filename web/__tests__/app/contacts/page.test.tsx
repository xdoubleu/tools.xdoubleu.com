import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const updateContact = jest.fn().mockResolvedValue({})
const deleteContact = jest.fn().mockResolvedValue({})

jest.mock('@/hooks/useContacts', () => ({
  useContacts: () => ({
    data: {
      contacts: [{ id: 'c1', displayName: 'Alice', contactUserId: 'u-alice' }],
      pending: [],
      incoming: []
    },
    isLoading: false,
    error: undefined
  }),
  useCreateContact: () => jest.fn(),
  useAcceptContact: () => jest.fn(),
  useDeclineContact: () => jest.fn(),
  useUpdateContact: () => updateContact,
  useDeleteContact: () => deleteContact
}))

jest.mock('swr', () => ({ __esModule: true, mutate: jest.fn() }))

import ContactsPage from '@/app/contacts/page'

beforeEach(() => jest.clearAllMocks())

describe('ContactsPage rename', () => {
  it('renames an accepted contact', async () => {
    render(<ContactsPage />)

    fireEvent.click(screen.getByRole('button', { name: 'Rename' }))
    const input = screen.getByLabelText('Rename Alice')
    fireEvent.change(input, { target: { value: 'Alice B.' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(updateContact).toHaveBeenCalledWith('c1', 'Alice B.'))
  })

  it('cancels editing without saving', () => {
    render(<ContactsPage />)
    fireEvent.click(screen.getByRole('button', { name: 'Rename' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.getByText('Alice')).toBeInTheDocument()
    expect(updateContact).not.toHaveBeenCalled()
  })
})
