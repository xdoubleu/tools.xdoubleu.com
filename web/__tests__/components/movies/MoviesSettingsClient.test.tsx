import React from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockSetServices = jest.fn()

jest.mock('@/hooks/useMovies', () => ({
  useMovieSettings: jest.fn(),
  useAvailableProviders: jest.fn(),
  useMoviesActions: jest.fn(() => ({ setServices: mockSetServices }))
}))

jest.mock('next/link', () => {
  const Link = ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
  return Object.assign(Link, { useLinkStatus: () => ({ pending: false }) })
})

jest.mock('next/image', () => ({
  __esModule: true,
  default: ({ src }: { src: string }) => (
    // eslint-disable-next-line @next/next/no-img-element -- next/image stand-in
    <img src={src} alt="" />
  )
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
  jest.mocked(useMovieSettings).mockReturnValue({
    data: opts.settingsLoading
      ? undefined
      : create(GetSettingsResponseSchema, { providerIds: opts.picked ?? [] }),
    isLoading: !!opts.settingsLoading,
    error: opts.settingsError
  })
  // @ts-expect-error -- partial SWRResponse
  jest.mocked(useAvailableProviders).mockReturnValue({
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

beforeEach(() => jest.clearAllMocks())

describe('MoviesSettingsClient', () => {
  it('lists the providers with the picked ones pressed', () => {
    mockHooks({ picked: [8n] })
    render(<MoviesSettingsClient />)

    expect(screen.getByRole('button', { name: 'Netflix' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Prime Video' })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
    expect(screen.getByRole('link', { name: /Movies & Series/ })).toBeInTheDocument()
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

    mockHooks({ settingsError: new Error('boom') })
    rerender(<MoviesSettingsClient />)
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })
})
