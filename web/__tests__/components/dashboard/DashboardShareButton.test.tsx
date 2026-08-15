import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  GetDashboardShareResponseSchema,
  DashboardShareSchema
} from '@/lib/gen/dashboard/v1/dashboard_pb'

const mockUseCurrentUser = jest.fn()
const mockUseDashboardShare = jest.fn()
const mockCreateShare = jest.fn()
const mockDeleteShare = jest.fn()
const mockMutate = jest.fn()

jest.mock('@/hooks/useAuth', () => ({
  useCurrentUser: () => mockUseCurrentUser()
}))

jest.mock('@/hooks/useDashboardShare', () => ({
  useDashboardShare: (kind: string) => mockUseDashboardShare(kind),
  useCreateDashboardShare: () => mockCreateShare,
  useDeleteDashboardShare: () => mockDeleteShare
}))

import DashboardShareButton from '@/components/dashboard/DashboardShareButton'

function withShare(token: string) {
  return create(GetDashboardShareResponseSchema, {
    share: create(DashboardShareSchema, { token, createdAt: '2026-01-01T00:00:00Z' })
  })
}

describe('DashboardShareButton', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockCreateShare.mockResolvedValue({})
    mockDeleteShare.mockResolvedValue({})
    mockUseDashboardShare.mockReturnValue({
      data: create(GetDashboardShareResponseSchema, {}),
      mutate: mockMutate
    })
  })

  function openDialog() {
    fireEvent.click(screen.getByRole('button', { name: 'Share dashboard' }))
  }

  it('prompts to set a display name before sharing is possible', () => {
    mockUseCurrentUser.mockReturnValue({ data: { displayName: '' } })
    render(<DashboardShareButton kind="reading" />)
    openDialog()

    expect(screen.getByText(/Set a display name in settings before sharing/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Create share link' })).not.toBeInTheDocument()
  })

  it('offers to create a link once a display name is set', () => {
    mockUseCurrentUser.mockReturnValue({ data: { displayName: 'Alice' } })
    render(<DashboardShareButton kind="reading" />)
    openDialog()

    expect(screen.getByRole('button', { name: 'Create share link' })).toBeInTheDocument()
  })

  it('creates a share for the given dashboard when the create button is clicked', async () => {
    mockUseCurrentUser.mockReturnValue({ data: { displayName: 'Alice' } })
    render(<DashboardShareButton kind="games" />)
    openDialog()

    fireEvent.click(screen.getByRole('button', { name: 'Create share link' }))

    await waitFor(() => {
      expect(mockCreateShare).toHaveBeenCalled()
    })
    expect(mockUseDashboardShare).toHaveBeenCalledWith('games')
    expect(mockMutate).toHaveBeenCalled()
  })

  it('shows the dashboard-scoped share URL with copy, regenerate, and disable controls', () => {
    mockUseCurrentUser.mockReturnValue({ data: { displayName: 'Alice' } })
    mockUseDashboardShare.mockReturnValue({ data: withShare('tok-123'), mutate: mockMutate })
    render(<DashboardShareButton kind="games" />)
    openDialog()

    const input = screen.getByLabelText('Public dashboard link') as HTMLInputElement
    expect(input.value).toContain('/dashboard/games/tok-123')
    expect(screen.getByRole('button', { name: 'Copy link' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Regenerate link' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Disable sharing' })).toBeInTheDocument()
  })

  it('copies the link to the clipboard', async () => {
    const writeText = jest.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
    mockUseCurrentUser.mockReturnValue({ data: { displayName: 'Alice' } })
    mockUseDashboardShare.mockReturnValue({ data: withShare('tok-123'), mutate: mockMutate })
    render(<DashboardShareButton kind="reading" />)
    openDialog()

    fireEvent.click(screen.getByRole('button', { name: 'Copy link' }))

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(expect.stringContaining('/dashboard/reading/tok-123'))
    })
    expect(await screen.findByRole('button', { name: 'Copied!' })).toBeInTheDocument()
  })

  it('regenerates the share', async () => {
    mockUseCurrentUser.mockReturnValue({ data: { displayName: 'Alice' } })
    mockUseDashboardShare.mockReturnValue({ data: withShare('tok-123'), mutate: mockMutate })
    render(<DashboardShareButton kind="reading" />)
    openDialog()

    fireEvent.click(screen.getByRole('button', { name: 'Regenerate link' }))

    await waitFor(() => {
      expect(mockCreateShare).toHaveBeenCalled()
    })
  })

  it('disables sharing', async () => {
    mockUseCurrentUser.mockReturnValue({ data: { displayName: 'Alice' } })
    mockUseDashboardShare.mockReturnValue({ data: withShare('tok-123'), mutate: mockMutate })
    render(<DashboardShareButton kind="reading" />)
    openDialog()

    fireEvent.click(screen.getByRole('button', { name: 'Disable sharing' }))

    await waitFor(() => {
      expect(mockDeleteShare).toHaveBeenCalled()
    })
    expect(mockMutate).toHaveBeenCalled()
  })
})
