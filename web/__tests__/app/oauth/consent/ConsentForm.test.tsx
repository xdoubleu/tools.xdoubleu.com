import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ConsentForm from '@/app/oauth/consent/ConsentForm'

const mockApprove = jest.fn()
const mockDeny = jest.fn()

jest.mock('@/app/oauth/consent/actions', () => ({
  approveAuthorization: (query: string) => mockApprove(query),
  denyAuthorization: (query: string) => mockDeny(query)
}))

describe('ConsentForm', () => {
  beforeEach(() => {
    mockApprove.mockReset().mockResolvedValue(undefined)
    mockDeny.mockReset().mockResolvedValue(undefined)
  })

  it('renders the client name and human-readable scopes', () => {
    render(<ConsentForm requestQuery="client_id=c1" clientName="Claude CLI" scope="openid email" />)
    expect(screen.getByText('Authorize Claude CLI')).toBeInTheDocument()
    expect(screen.getByText('Verify your identity')).toBeInTheDocument()
    expect(screen.getByText('Read your email address')).toBeInTheDocument()
  })

  it('shows unknown scopes verbatim', () => {
    render(<ConsentForm requestQuery="client_id=c1" clientName="X" scope="custom:scope" />)
    expect(screen.getByText('custom:scope')).toBeInTheDocument()
  })

  it('approves with the original request query', async () => {
    render(<ConsentForm requestQuery="client_id=c1&scope=openid" clientName="X" scope="openid" />)
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    await waitFor(() => expect(mockApprove).toHaveBeenCalledWith('client_id=c1&scope=openid'))
    expect(mockDeny).not.toHaveBeenCalled()
  })

  it('denies with the original request query', async () => {
    render(<ConsentForm requestQuery="client_id=c1&scope=openid" clientName="X" scope="openid" />)
    fireEvent.click(screen.getByRole('button', { name: 'Deny' }))
    await waitFor(() => expect(mockDeny).toHaveBeenCalledWith('client_id=c1&scope=openid'))
  })

  it('surfaces an error when the action rejects', async () => {
    mockApprove.mockRejectedValue(new Error('boom'))
    render(<ConsentForm requestQuery="client_id=c1" clientName="X" scope="openid" />)
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    await waitFor(() =>
      expect(screen.getByText('Something went wrong. Please try again.')).toBeInTheDocument()
    )
  })
})
