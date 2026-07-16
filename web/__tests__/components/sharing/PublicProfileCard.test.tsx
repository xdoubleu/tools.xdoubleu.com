import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { GetProfileShareResponseSchema, ProfileShareSchema } from '@/lib/gen/profile/v1/profile_pb'

const mockUseProfileShare = jest.fn()
const mockCreateShare = jest.fn()
const mockDeleteShare = jest.fn()
const mockMutate = jest.fn()

jest.mock('@/hooks/useProfile', () => ({
  useProfileShare: () => mockUseProfileShare(),
  useCreateProfileShare: () => mockCreateShare,
  useDeleteProfileShare: () => mockDeleteShare
}))

import PublicProfileCard from '@/components/sharing/PublicProfileCard'

function withShare(token: string) {
  return create(GetProfileShareResponseSchema, {
    share: create(ProfileShareSchema, { token, createdAt: '2026-01-01T00:00:00Z' })
  })
}

describe('PublicProfileCard', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockCreateShare.mockResolvedValue({})
    mockDeleteShare.mockResolvedValue({})
  })

  it('offers to create a link when no share exists', () => {
    mockUseProfileShare.mockReturnValue({
      data: create(GetProfileShareResponseSchema, {}),
      mutate: mockMutate
    })
    render(<PublicProfileCard />)
    expect(screen.getByRole('button', { name: 'Create share link' })).toBeInTheDocument()
  })

  it('creates a share when the create button is clicked', async () => {
    mockUseProfileShare.mockReturnValue({
      data: create(GetProfileShareResponseSchema, {}),
      mutate: mockMutate
    })
    render(<PublicProfileCard />)

    fireEvent.click(screen.getByRole('button', { name: 'Create share link' }))

    await waitFor(() => {
      expect(mockCreateShare).toHaveBeenCalled()
    })
    expect(mockMutate).toHaveBeenCalled()
  })

  it('shows the share URL with copy, regenerate, and disable controls', () => {
    mockUseProfileShare.mockReturnValue({ data: withShare('tok-123'), mutate: mockMutate })
    render(<PublicProfileCard />)

    const input = screen.getByLabelText('Public profile link') as HTMLInputElement
    expect(input.value).toContain('/profile/tok-123')
    expect(screen.getByRole('button', { name: 'Copy link' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Regenerate link' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Disable sharing' })).toBeInTheDocument()
  })

  it('copies the link to the clipboard', async () => {
    const writeText = jest.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
    mockUseProfileShare.mockReturnValue({ data: withShare('tok-123'), mutate: mockMutate })
    render(<PublicProfileCard />)

    fireEvent.click(screen.getByRole('button', { name: 'Copy link' }))

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(expect.stringContaining('/profile/tok-123'))
    })
    expect(await screen.findByRole('button', { name: 'Copied!' })).toBeInTheDocument()
  })

  it('regenerates the share', async () => {
    mockUseProfileShare.mockReturnValue({ data: withShare('tok-123'), mutate: mockMutate })
    render(<PublicProfileCard />)

    fireEvent.click(screen.getByRole('button', { name: 'Regenerate link' }))

    await waitFor(() => {
      expect(mockCreateShare).toHaveBeenCalled()
    })
  })

  it('disables sharing', async () => {
    mockUseProfileShare.mockReturnValue({ data: withShare('tok-123'), mutate: mockMutate })
    render(<PublicProfileCard />)

    fireEvent.click(screen.getByRole('button', { name: 'Disable sharing' }))

    await waitFor(() => {
      expect(mockDeleteShare).toHaveBeenCalled()
    })
    expect(mockMutate).toHaveBeenCalled()
  })
})
