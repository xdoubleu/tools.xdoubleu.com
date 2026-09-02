import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const inviteToFamily = jest.fn().mockResolvedValue({})
const acceptFamilyInvite = jest.fn().mockResolvedValue({})
const declineFamilyInvite = jest.fn().mockResolvedValue({})
const leaveFamily = jest.fn().mockResolvedValue({})

let mockData: {
  members: { userId: string; email: string }[]
  incomingInvite?: { fromUserId: string; fromEmail: string } | undefined
} = { members: [], incomingInvite: undefined }
let mockIsLoading = false
let mockError: Error | undefined

jest.mock('@/hooks/useFamily', () => ({
  useFamily: () => ({
    data: mockData,
    isLoading: mockIsLoading,
    error: mockError
  }),
  useInviteToFamily: () => inviteToFamily,
  useAcceptFamilyInvite: () => acceptFamilyInvite,
  useDeclineFamilyInvite: () => declineFamilyInvite,
  useLeaveFamily: () => leaveFamily
}))

jest.mock('swr', () => ({ __esModule: true, mutate: jest.fn() }))

import FamilyPageClient from '@/components/family/FamilyPageClient'

beforeEach(() => {
  jest.clearAllMocks()
  mockData = { members: [], incomingInvite: undefined }
  mockIsLoading = false
  mockError = undefined
})

describe('FamilyPageClient', () => {
  it('shows a loading state', () => {
    mockIsLoading = true
    render(<FamilyPageClient />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    mockError = new Error('boom')
    render(<FamilyPageClient />)
    expect(screen.getByText('Failed to load family.')).toBeInTheDocument()
  })

  it('shows a solo-family message when there are no members', () => {
    render(<FamilyPageClient />)
    expect(screen.getByText(/Just you for now/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Leave family/ })).not.toBeInTheDocument()
  })

  it('lists members and offers to leave', () => {
    mockData = {
      members: [{ userId: 'u1', email: 'partner@example.com' }],
      incomingInvite: undefined
    }
    render(<FamilyPageClient />)
    expect(screen.getByText('partner@example.com')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Leave family/ })).toBeInTheDocument()
  })

  it('sends an invite by email', async () => {
    render(<FamilyPageClient />)
    fireEvent.change(screen.getByPlaceholderText('Email address'), {
      target: { value: 'friend@example.com' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Invite' }))

    await waitFor(() => expect(inviteToFamily).toHaveBeenCalledWith('friend@example.com'))
  })

  it('shows an error when the invite fails', async () => {
    inviteToFamily.mockRejectedValueOnce(new Error('nope'))
    render(<FamilyPageClient />)
    fireEvent.change(screen.getByPlaceholderText('Email address'), {
      target: { value: 'friend@example.com' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Invite' }))

    await waitFor(() =>
      expect(screen.getByText('Failed to send invite. Check the email and try again.'))
        .toBeInTheDocument()
    )
  })

  it('accepts an incoming invite', async () => {
    mockData = {
      members: [],
      incomingInvite: { fromUserId: 'u2', fromEmail: 'sender@example.com' }
    }
    render(<FamilyPageClient />)
    expect(screen.getByText(/sender@example.com invited you/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Accept' }))
    await waitFor(() => expect(acceptFamilyInvite).toHaveBeenCalled())
  })

  it('declines an incoming invite', async () => {
    mockData = {
      members: [],
      incomingInvite: { fromUserId: 'u2', fromEmail: 'sender@example.com' }
    }
    render(<FamilyPageClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Decline' }))
    await waitFor(() => expect(declineFamilyInvite).toHaveBeenCalled())
  })

  it('leaves the family', async () => {
    mockData = {
      members: [{ userId: 'u1', email: 'partner@example.com' }],
      incomingInvite: undefined
    }
    render(<FamilyPageClient />)

    fireEvent.click(screen.getByRole('button', { name: /Leave family/ }))
    await waitFor(() => expect(leaveFamily).toHaveBeenCalled())
  })
})
