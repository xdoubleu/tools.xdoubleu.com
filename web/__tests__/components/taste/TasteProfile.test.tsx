import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'

const mockRefresh = jest.fn()

jest.mock('@/hooks/useTaste', () => ({
  useTasteProfile: jest.fn(),
  useRefreshTasteProfile: jest.fn(() => mockRefresh)
}))

import TasteProfile from '@/components/taste/TasteProfile'
import { useTasteProfile } from '@/hooks/useTaste'
import { GetProfileResponseSchema, ProfileSectionsSchema } from '@/lib/gen/taste/v1/taste_pb'

function mockProfile(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  jest.mocked(useTasteProfile).mockReturnValue({ isLoading: false, ...value })
}

const sections = create(ProfileSectionsSchema, {
  summary: 'You love big ideas.',
  know: ['Space opera'],
  like: ['World-building', 'Slow burns'],
  drifting: ['Toward essays'],
  bounce: []
})

const written = create(GetProfileResponseSchema, {
  configured: true,
  sections,
  model: 'anthropic/claude-sonnet-5',
  generatedAt: '2026-10-01T10:00:00Z'
})

beforeEach(() => jest.clearAllMocks())

describe('TasteProfile', () => {
  it('shows loading and error states without data', () => {
    mockProfile({ isLoading: true })
    const { rerender } = render(<TasteProfile />)
    expect(screen.getByText(/Loading profile/)).toBeInTheDocument()

    mockProfile({ error: new Error('nope') })
    rerender(<TasteProfile />)
    expect(screen.getByRole('alert')).toBeInTheDocument()

    mockProfile({})
    const { container } = render(<TasteProfile />)
    expect(container).toBeEmptyDOMElement()
  })

  it('explains a missing API key', () => {
    mockProfile({ data: create(GetProfileResponseSchema, { configured: false }) })
    render(<TasteProfile />)
    expect(screen.getByText(/need an OpenRouter API key/)).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('keeps repeated bullets apart', () => {
    const spy = jest.spyOn(console, 'error').mockImplementation(() => {})
    mockProfile({
      data: create(GetProfileResponseSchema, {
        ...written,
        sections: create(ProfileSectionsSchema, { summary: 'S', like: ['Same', 'Same'] })
      })
    })
    render(<TasteProfile />)
    expect(screen.getAllByText('Same')).toHaveLength(2)
    expect(spy).not.toHaveBeenCalled()
    spy.mockRestore()
  })

  it('shows the profile sections, skipping empty ones', () => {
    mockProfile({ data: written })
    render(<TasteProfile />)
    expect(screen.getByText('You love big ideas.')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'What you know' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'What you like' })).toBeInTheDocument()
    expect(screen.getByText('Slow burns')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Drifting toward' })).toBeInTheDocument()
    expect(screen.getByText('Toward essays')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Bounce off' })).not.toBeInTheDocument()
    expect(screen.getByText(/Written .* by anthropic\/claude-sonnet-5/)).toBeInTheDocument()
    expect(screen.queryByText(/No profile yet/)).not.toBeInTheDocument()
    // Nothing failed and no cooldown: no warning, error or wait hint.
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.queryByText(/Again after/)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeEnabled()
  })

  it('invites a first refresh', async () => {
    mockRefresh.mockResolvedValue(undefined)
    mockProfile({ data: create(GetProfileResponseSchema, { configured: true }) })
    render(<TasteProfile />)
    expect(screen.getByText(/No profile yet/)).toBeInTheDocument()
    expect(screen.queryByText(/Written/)).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(screen.getByRole('button', { name: 'Refreshing…' })).toBeDisabled()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Refresh' })).toBeEnabled())
    expect(mockRefresh).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows a pending regeneration', () => {
    mockProfile({
      data: create(GetProfileResponseSchema, {
        configured: true,
        pending: true,
        nextRefreshAt: '2026-10-12T16:00:00Z'
      })
    })
    render(<TasteProfile />)
    expect(screen.getByText('Writing your profile…')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Refreshing…' })).toBeDisabled()
    expect(screen.queryByText(/Again after/)).not.toBeInTheDocument()
    expect(screen.queryByText(/No profile yet/)).not.toBeInTheDocument()
  })

  it('waits out the cooldown and shows the last failure', () => {
    mockProfile({
      data: create(GetProfileResponseSchema, {
        ...written,
        lastError: 'The model service failed.',
        nextRefreshAt: '2026-10-12T16:00:00Z'
      })
    })
    render(<TasteProfile />)
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeDisabled()
    expect(screen.getByText(/Again after/)).toBeInTheDocument()
    expect(screen.getByText('The model service failed.')).toBeInTheDocument()
  })

  it('shows a refused refresh', async () => {
    mockRefresh.mockRejectedValue(new ConnectError('too soon', Code.ResourceExhausted))
    mockProfile({ data: written })
    render(<TasteProfile />)
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('too soon'))
  })
})
