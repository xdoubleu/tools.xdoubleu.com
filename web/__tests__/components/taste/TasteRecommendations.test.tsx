import React from 'react'
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

jest.mock('@/hooks/useTaste', () => ({
  useTasteRecommendations: jest.fn(),
  useSetEmailNewPicks: jest.fn()
}))
jest.mock('@/components/taste/PickActions', () => ({
  __esModule: true,
  default: ({ pick }: { pick: { id: string } }) => <div data-testid="actions" data-id={pick.id} />
}))
jest.mock('@/components/taste/PickArtwork', () => ({
  __esModule: true,
  default: ({ url, title }: { url: string; title: string }) => (
    <div data-testid="artwork" data-url={url} data-title={title} />
  )
}))

import TasteRecommendations from '@/components/taste/TasteRecommendations'
import { useSetEmailNewPicks, useTasteRecommendations } from '@/hooks/useTaste'
import {
  ListRecommendationsResponseSchema,
  RecommendationSchema,
  SourceSchema
} from '@/lib/gen/taste/v1/taste_pb'

function mockPicks(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  jest.mocked(useTasteRecommendations).mockReturnValue({ isLoading: false, ...value })
}

const hyperion = create(RecommendationSchema, {
  id: 'r1',
  domain: 'books',
  track: 'more',
  title: 'Hyperion',
  creator: 'Dan Simmons',
  year: 1989,
  url: 'https://cat.example/hyperion',
  imageUrl: 'https://img.example/h.jpg',
  why: 'You loved Dune.',
  sources: [
    create(SourceSchema, { url: 'https://www.reddit.com/r/printSF', title: 'r/printSF' }),
    create(SourceSchema, { url: 'https://a.example/1' }),
    create(SourceSchema, { url: 'https://b.example/2' }),
    create(SourceSchema, { url: 'https://c.example/3' })
  ],
  score: 88
})
const bare = create(RecommendationSchema, { id: 'r2', domain: 'podcasts', title: 'Radiolab' })

let setEmailNewPicks: jest.Mock

beforeEach(() => {
  jest.clearAllMocks()
  setEmailNewPicks = jest.fn().mockResolvedValue(undefined)
  jest.mocked(useSetEmailNewPicks).mockReturnValue(setEmailNewPicks)
})

describe('TasteRecommendations', () => {
  it('shows a pick with its artwork, meta, why, link and up to three sources', () => {
    mockPicks({
      data: create(ListRecommendationsResponseSchema, {
        recommendations: [hyperion, bare],
        generatedAt: '2026-10-12T05:00:00Z'
      })
    })
    render(<TasteRecommendations />)
    const list = screen.getByRole('list', { name: 'Recommendations' })
    const [first, second] = within(list).getAllByRole('listitem')

    expect(within(first).getByText('Hyperion')).toBeInTheDocument()
    expect(within(first).getByText('Books · Dan Simmons · 1989')).toBeInTheDocument()
    expect(within(first).getByText('You loved Dune.')).toBeInTheDocument()
    expect(within(first).getByTestId('artwork')).toHaveAttribute(
      'data-url',
      'https://img.example/h.jpg'
    )
    expect(within(first).getByRole('link', { name: 'Open' })).toHaveAttribute(
      'href',
      'https://cat.example/hyperion'
    )
    expect(within(first).getByRole('link', { name: 'via reddit.com' })).toHaveClass(
      'min-h-11',
      'break-all'
    )
    expect(within(first).getByRole('link', { name: 'Open' })).toHaveClass('min-h-11', 'min-w-11')
    expect(within(first).getByRole('link', { name: 'via b.example' })).toBeInTheDocument()
    expect(within(first).queryByRole('link', { name: 'via c.example' })).not.toBeInTheDocument()

    expect(within(first).getByTestId('actions')).toHaveAttribute('data-id', 'r1')
    expect(within(first).queryByText('On your services')).not.toBeInTheDocument()
    expect(within(second).getByText('Podcasts')).toBeInTheDocument()
    expect(within(second).queryByRole('link')).not.toBeInTheDocument()
    expect(screen.getByText(/Picked .*, each checked against its catalog\./)).toBeInTheDocument()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('keeps repeated source links apart', () => {
    const spy = jest.spyOn(console, 'error').mockImplementation(() => {})
    const twice = create(SourceSchema, { url: 'https://a.example/1' })
    mockPicks({
      data: create(ListRecommendationsResponseSchema, {
        recommendations: [create(RecommendationSchema, { ...bare, sources: [twice, twice] })]
      })
    })
    render(<TasteRecommendations />)
    expect(screen.getAllByRole('link', { name: 'via a.example' })).toHaveLength(2)
    expect(spy).not.toHaveBeenCalled()
    spy.mockRestore()
  })

  it('switches track and domain', () => {
    mockPicks({ data: create(ListRecommendationsResponseSchema, {}) })
    render(<TasteRecommendations />)
    expect(useTasteRecommendations).toHaveBeenLastCalledWith('', 'more')
    expect(screen.getByRole('tab', { name: 'More of what you like' })).toHaveAttribute(
      'aria-selected',
      'true'
    )

    fireEvent.click(screen.getByRole('tab', { name: 'Broaden your horizon' }))
    expect(useTasteRecommendations).toHaveBeenLastCalledWith('', 'broaden')
    fireEvent.change(screen.getByLabelText('Domain'), { target: { value: 'games' } })
    expect(useTasteRecommendations).toHaveBeenLastCalledWith('games', 'broaden')
  })

  it('lists every saved pick with its track, without the run time', () => {
    mockPicks({
      data: create(ListRecommendationsResponseSchema, {
        recommendations: [
          create(RecommendationSchema, {
            ...hyperion,
            sources: [],
            saved: true,
            onMyServices: true,
            track: 'broaden'
          })
        ],
        generatedAt: '2026-10-12T05:00:00Z'
      })
    })
    render(<TasteRecommendations />)
    fireEvent.click(screen.getByRole('tab', { name: 'Saved' }))
    expect(useTasteRecommendations).toHaveBeenLastCalledWith('', 'saved')
    expect(
      screen.getByText('Books · Broaden your horizon · Dan Simmons · 1989')
    ).toBeInTheDocument()
    expect(screen.getByText('On your services')).toBeInTheDocument()
    expect(screen.queryByText(/Picked/)).not.toBeInTheDocument()

    mockPicks({ data: create(ListRecommendationsResponseSchema, {}) })
    fireEvent.change(screen.getByLabelText('Domain'), { target: { value: 'games' } })
    expect(screen.getByText(/Nothing saved yet/)).toBeInTheDocument()
  })

  it('shows empty, failed, loading and error states', () => {
    mockPicks({
      data: create(ListRecommendationsResponseSchema, { lastError: 'The model service failed.' })
    })
    const { rerender } = render(<TasteRecommendations />)
    expect(screen.getByText(/No picks yet/)).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('The model service failed.')
    expect(screen.queryByText(/Picked/)).not.toBeInTheDocument()

    mockPicks({ isLoading: true })
    rerender(<TasteRecommendations />)
    expect(screen.getByText(/Loading recommendations/)).toBeInTheDocument()

    mockPicks({ error: new Error('nope') })
    rerender(<TasteRecommendations />)
    expect(screen.getByRole('alert')).toBeInTheDocument()

    mockPicks({})
    rerender(<TasteRecommendations />)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.queryByText(/Loading/)).not.toBeInTheDocument()
    expect(screen.queryByText(/No picks yet/)).not.toBeInTheDocument()
  })

  it('keeps loaded picks after a failed refresh', () => {
    mockPicks({
      data: create(ListRecommendationsResponseSchema, { recommendations: [hyperion] }),
      error: new Error('nope'),
      isLoading: true
    })
    render(<TasteRecommendations />)
    expect(screen.getByText('Hyperion')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows the email setting and turns it off', async () => {
    mockPicks({ data: create(ListRecommendationsResponseSchema, { emailNewPicks: true }) })
    render(<TasteRecommendations />)
    const box = screen.getByRole('checkbox', { name: 'Email me when new picks are ready' })
    expect(box).toBeChecked()

    await act(async () => {
      fireEvent.click(box)
    })
    expect(setEmailNewPicks).toHaveBeenCalledWith(false)
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('shows the setting off and turns it on', async () => {
    mockPicks({ data: create(ListRecommendationsResponseSchema, { emailNewPicks: false }) })
    render(<TasteRecommendations />)
    const box = screen.getByRole('checkbox', { name: 'Email me when new picks are ready' })
    expect(box).not.toBeChecked()

    await act(async () => {
      fireEvent.click(box)
    })
    expect(setEmailNewPicks).toHaveBeenCalledWith(true)
  })

  it('says so when the setting could not be saved', async () => {
    setEmailNewPicks.mockRejectedValueOnce(new Error('nope'))
    mockPicks({ data: create(ListRecommendationsResponseSchema, { emailNewPicks: true }) })
    render(<TasteRecommendations />)

    await act(async () => {
      fireEvent.click(screen.getByRole('checkbox', { name: 'Email me when new picks are ready' }))
    })
    expect(screen.getByRole('status')).toHaveTextContent("Couldn't save that")
    expect(screen.getByRole('checkbox', { name: /Email me/ })).not.toBeDisabled()
  })
})
