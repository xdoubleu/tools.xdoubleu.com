import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { GetProfileShareResponseSchema, ProfileShareSchema } from '@/lib/gen/profile/v1/profile_pb'

const mockUseCurrentUser = jest.fn()
const mockUseProfileShare = jest.fn()
const mockCreateShare = jest.fn()
const mockDeleteShare = jest.fn()
const mockMutate = jest.fn()

jest.mock('@/hooks/useAuth', () => ({
  useCurrentUser: () => mockUseCurrentUser()
}))

jest.mock('@/hooks/useProfile', () => ({
  useProfileShare: (app: string) => mockUseProfileShare(app),
  useCreateProfileShare: () => mockCreateShare,
  useDeleteProfileShare: () => mockDeleteShare
}))

import ProfileShareButton from '@/components/profile/ProfileShareButton'

function withShare(token: string) {
  return create(GetProfileShareResponseSchema, {
    share: create(ProfileShareSchema, { token, createdAt: '2026-01-01T00:00:00Z' })
  })
}

describe('ProfileShareButton', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockCreateShare.mockResolvedValue({})
    mockDeleteShare.mockResolvedValue({})
    mockUseProfileShare.mockReturnValue({
      data: create(GetProfileShareResponseSchema, {}),
      mutate: mockMutate
    })
  })

  function openDialog() {
    fireEvent.click(screen.getByRole('button', { name: 'Share profile' }))
  }

  it('prompts to set a display name before sharing is possible', () => {
    mockUseCurrentUser.mockReturnValue({ data: { displayName: '' } })
    render(<ProfileShareButton app="books" />)
    openDialog()

    expect(screen.getByText(/Set a display name in settings before sharing/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Create share link' })).not.toBeInTheDocument()
  })

  it('offers to create a link once a display name is set', () => {
    mockUseCurrentUser.mockReturnValue({ data: { displayName: 'Alice' } })
    render(<ProfileShareButton app="books" />)
    openDialog()

    expect(screen.getByRole('button', { name: 'Create share link' })).toBeInTheDocument()
  })

  it('creates a share for the given app when the create button is clicked', async () => {
    mockUseCurrentUser.mockReturnValue({ data: { displayName: 'Alice' } })
    render(<ProfileShareButton app="games" />)
    openDialog()

    fireEvent.click(screen.getByRole('button', { name: 'Create share link' }))

    await waitFor(() => {
      expect(mockCreateShare).toHaveBeenCalled()
    })
    expect(mockUseProfileShare).toHaveBeenCalledWith('games')
    expect(mockMutate).toHaveBeenCalled()
  })

  it('shows the app-scoped share URL with copy, regenerate, and disable controls', () => {
    mockUseCurrentUser.mockReturnValue({ data: { displayName: 'Alice' } })
    mockUseProfileShare.mockReturnValue({ data: withShare('tok-123'), mutate: mockMutate })
    render(<ProfileShareButton app="games" />)
    openDialog()

    const input = screen.getByLabelText('Public profile link') as HTMLInputElement
    expect(input.value).toContain('/profile/games/tok-123')
    expect(screen.getByRole('button', { name: 'Copy link' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Regenerate link' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Disable sharing' })).toBeInTheDocument()
  })

  it('copies the link to the clipboard', async () => {
    const writeText = jest.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
    mockUseCurrentUser.mockReturnValue({ data: { displayName: 'Alice' } })
    mockUseProfileShare.mockReturnValue({ data: withShare('tok-123'), mutate: mockMutate })
    render(<ProfileShareButton app="books" />)
    openDialog()

    fireEvent.click(screen.getByRole('button', { name: 'Copy link' }))

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(expect.stringContaining('/profile/books/tok-123'))
    })
    expect(await screen.findByRole('button', { name: 'Copied!' })).toBeInTheDocument()
  })

  it('regenerates the share', async () => {
    mockUseCurrentUser.mockReturnValue({ data: { displayName: 'Alice' } })
    mockUseProfileShare.mockReturnValue({ data: withShare('tok-123'), mutate: mockMutate })
    render(<ProfileShareButton app="books" />)
    openDialog()

    fireEvent.click(screen.getByRole('button', { name: 'Regenerate link' }))

    await waitFor(() => {
      expect(mockCreateShare).toHaveBeenCalled()
    })
  })

  it('disables sharing', async () => {
    mockUseCurrentUser.mockReturnValue({ data: { displayName: 'Alice' } })
    mockUseProfileShare.mockReturnValue({ data: withShare('tok-123'), mutate: mockMutate })
    render(<ProfileShareButton app="books" />)
    openDialog()

    fireEvent.click(screen.getByRole('button', { name: 'Disable sharing' }))

    await waitFor(() => {
      expect(mockDeleteShare).toHaveBeenCalled()
    })
    expect(mockMutate).toHaveBeenCalled()
  })
})
