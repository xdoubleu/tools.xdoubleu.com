import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ConsentForm from '@/app/oauth/consent/ConsentForm'

const mockApprove = jest.fn()
const mockDeny = jest.fn()

jest.mock('@/app/oauth/consent/actions', () => ({
  approveAuthorization: (id: string) => mockApprove(id),
  denyAuthorization: (id: string) => mockDeny(id)
}))

describe('ConsentForm', () => {
  beforeEach(() => {
    mockApprove.mockReset().mockResolvedValue(undefined)
    mockDeny.mockReset().mockResolvedValue(undefined)
  })

  it('renders the client name and human-readable scopes', () => {
    render(<ConsentForm authorizationId="auth-1" clientName="Claude CLI" scope="openid email" />)
    expect(screen.getByText('Authorize Claude CLI')).toBeInTheDocument()
    expect(screen.getByText('Verify your identity')).toBeInTheDocument()
    expect(screen.getByText('Read your email address')).toBeInTheDocument()
  })

  it('shows unknown scopes verbatim', () => {
    render(<ConsentForm authorizationId="auth-1" clientName="X" scope="custom:scope" />)
    expect(screen.getByText('custom:scope')).toBeInTheDocument()
  })

  it('approves with the authorization id', async () => {
    render(<ConsentForm authorizationId="auth-1" clientName="X" scope="openid" />)
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    await waitFor(() => expect(mockApprove).toHaveBeenCalledWith('auth-1'))
    expect(mockDeny).not.toHaveBeenCalled()
  })

  it('denies with the authorization id', async () => {
    render(<ConsentForm authorizationId="auth-1" clientName="X" scope="openid" />)
    fireEvent.click(screen.getByRole('button', { name: 'Deny' }))
    await waitFor(() => expect(mockDeny).toHaveBeenCalledWith('auth-1'))
  })

  it('surfaces an error when the action rejects', async () => {
    mockApprove.mockRejectedValue(new Error('boom'))
    render(<ConsentForm authorizationId="auth-1" clientName="X" scope="openid" />)
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    await waitFor(() =>
      expect(screen.getByText('Something went wrong. Please try again.')).toBeInTheDocument()
    )
  })
})
