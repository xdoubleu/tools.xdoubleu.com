import React from 'react'
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockSetServices = vi.fn()

vi.mock('@/hooks/useMovies', () => ({
  useMovieSettings: vi.fn(),
  useAvailableProviders: vi.fn(),
  useMoviesActions: vi.fn(() => ({ setServices: mockSetServices }))
}))

import MoviesSettingsClient from '@/components/movies/MoviesSettingsClient'
import { useAvailableProviders, useMovieSettings } from '@/hooks/useMovies'
import {
  GetSettingsResponseSchema,
  ListAvailableProvidersResponseSchema,
  ProviderSchema
} from '@/lib/gen/movies/v1/movies_pb'

function mockHooks(opts: {
  picked?: bigint[]
  settingsLoading?: boolean
  settingsError?: Error
  providersLoading?: boolean
  providersError?: Error
}) {
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useMovieSettings).mockReturnValue({
    data: opts.settingsLoading
      ? undefined
      : create(GetSettingsResponseSchema, { providerIds: opts.picked ?? [] }),
    isLoading: !!opts.settingsLoading,
    error: opts.settingsError
  })
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useAvailableProviders).mockReturnValue({
    data:
      opts.providersLoading || opts.providersError
        ? undefined
        : create(ListAvailableProvidersResponseSchema, {
            providers: [
              create(ProviderSchema, { id: 8n, name: 'Netflix', logoPath: '/n.jpg' }),
              create(ProviderSchema, { id: 119n, name: 'Prime Video' })
            ]
          }),
    isLoading: !!opts.providersLoading,
    error: opts.providersError
  })
}

beforeEach(() => vi.clearAllMocks())

describe('MoviesSettingsClient', () => {
  it('lists the providers with the picked ones pressed', () => {
    mockHooks({ picked: [8n] })
    render(<MoviesSettingsClient />)

    expect(screen.getByRole('button', { name: 'Netflix' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Prime Video' })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByRole('navigation')).toHaveTextContent('Movies & series/Settings')
    expect(screen.getByRole('link', { name: 'Movies & series' })).toHaveAttribute('href', '/movies')
  })

  it('shows a logo only for providers that have one', () => {
    mockHooks({})
    render(<MoviesSettingsClient />)

    expect(screen.getAllByRole('presentation')).toHaveLength(1)
    expect(
      within(screen.getByRole('button', { name: 'Netflix' })).getByRole('presentation')
    ).toBeInTheDocument()
  })

  it('locks the pills while saving', async () => {
    let resolve!: () => void
    mockSetServices.mockReturnValueOnce(new Promise<void>((r) => (resolve = r)))
    mockHooks({})
    render(<MoviesSettingsClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Netflix' }))
    expect(screen.getByRole('button', { name: 'Prime Video' })).toBeDisabled()

    await act(async () => resolve())
    expect(screen.getByRole('button', { name: 'Prime Video' })).toBeEnabled()
  })

  it('adds a service to the picked ones', async () => {
    mockSetServices.mockResolvedValue(undefined)
    mockHooks({ picked: [8n] })
    render(<MoviesSettingsClient />)

    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Prime Video' })))

    expect(mockSetServices).toHaveBeenCalledWith([8n, 119n])
  })

  it('removes a picked service', async () => {
    mockSetServices.mockResolvedValue(undefined)
    mockHooks({ picked: [8n, 119n] })
    render(<MoviesSettingsClient />)

    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Netflix' })))

    expect(mockSetServices).toHaveBeenCalledWith([119n])
  })

  it('says so when saving fails, and clears it on the next try', async () => {
    mockSetServices.mockRejectedValueOnce(new Error('nope')).mockResolvedValueOnce(undefined)
    mockHooks({})
    render(<MoviesSettingsClient />)

    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Netflix' })))
    expect(screen.getByText(/Couldn.t save/)).toBeInTheDocument()

    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Netflix' })))
    expect(screen.queryByText(/Couldn.t save/)).not.toBeInTheDocument()
  })

  it('shows loading and error states', () => {
    mockHooks({ settingsLoading: true })
    const { rerender } = render(<MoviesSettingsClient />)
    expect(screen.getByText(/Loading/)).toBeInTheDocument()

    mockHooks({ providersLoading: true })
    rerender(<MoviesSettingsClient />)
    expect(screen.getByText(/Loading/)).toBeInTheDocument()

    mockHooks({ providersError: new Error('boom') })
    rerender(<MoviesSettingsClient />)
    expect(screen.getByRole('alert')).toBeInTheDocument()

    mockHooks({ settingsLoading: true, settingsError: new Error('boom') })
    rerender(<MoviesSettingsClient />)
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })

  it('keeps the loaded picker when a revalidation fails', () => {
    mockHooks({ picked: [8n], settingsError: new Error('boom') })
    render(<MoviesSettingsClient />)

    expect(screen.getByRole('button', { name: 'Netflix' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
