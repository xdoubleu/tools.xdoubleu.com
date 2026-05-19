import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import AddContactForm from '@/components/contacts/AddContactForm'

jest.mock('@/hooks/useContacts', () => ({
  useCreateContact: jest.fn(() => jest.fn().mockResolvedValue({}))
}))

describe('AddContactForm', () => {
  const mockOnSuccess = jest.fn()

  beforeEach(() => {
    mockOnSuccess.mockClear()
  })

  it('renders form fields', () => {
    render(<AddContactForm />)
    expect(screen.getByLabelText('Email')).toBeInTheDocument()
    expect(screen.getByLabelText('Display Name (optional)')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add Contact' })).toBeInTheDocument()
  })

  it('submits form with email and display name', async () => {
    render(<AddContactForm onSuccess={mockOnSuccess} />)
    const emailInput = screen.getByLabelText('Email') as HTMLInputElement
    const nameInput = screen.getByLabelText('Display Name (optional)') as HTMLInputElement
    const submitBtn = screen.getByRole('button', { name: 'Add Contact' })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(nameInput, { target: { value: 'Test User' } })
    fireEvent.click(submitBtn)

    await waitFor(() => {
      expect(mockOnSuccess).toHaveBeenCalled()
    })
  })

  it('clears inputs after successful submission', async () => {
    render(<AddContactForm onSuccess={mockOnSuccess} />)
    const emailInput = screen.getByLabelText('Email') as HTMLInputElement
    const nameInput = screen.getByLabelText('Display Name (optional)') as HTMLInputElement
    const submitBtn = screen.getByRole('button', { name: 'Add Contact' })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(nameInput, { target: { value: 'Test User' } })
    fireEvent.click(submitBtn)

    await waitFor(() => {
      expect(emailInput.value).toBe('')
      expect(nameInput.value).toBe('')
    })
  })
})
