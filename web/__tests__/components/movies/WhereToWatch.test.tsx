import React from 'react'
import { render, screen, within } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

import WhereToWatch from '@/components/movies/WhereToWatch'
import { ProviderOffersSchema, ProviderSchema } from '@/lib/gen/movies/v1/movies_pb'

const offers = [
  create(ProviderOffersSchema, {
    offerType: 'flatrate',
    providers: [
      create(ProviderSchema, { id: 8n, name: 'Netflix', logoPath: '/n.jpg', mine: true }),
      create(ProviderSchema, { id: 119n, name: 'Prime Video' })
    ]
  }),
  create(ProviderOffersSchema, {
    offerType: 'rent',
    providers: [create(ProviderSchema, { id: 350n, name: 'Apple TV' })]
  })
]

describe('WhereToWatch', () => {
  it('groups providers by offer type with a logo when TMDB has one', () => {
    render(<WhereToWatch offers={offers} watchLink="https://tmdb.example/watch" />)

    const stream = screen.getByRole('heading', { name: 'Stream' }).parentElement!
    expect(within(stream).getByText('Netflix')).toBeInTheDocument()
    expect(within(stream).getByRole('presentation')).toHaveAttribute(
      'src',
      expect.stringContaining('image.tmdb.org/t/p/w92/n.jpg')
    )
    const rent = screen.getByRole('heading', { name: 'Rent' }).parentElement!
    expect(within(rent).getByText('Apple TV')).toBeInTheDocument()
  })

  it('marks only the providers on my services', () => {
    render(<WhereToWatch offers={offers} watchLink="" />)

    const stream = screen.getByRole('heading', { name: 'Stream' }).parentElement!
    expect(within(stream).getAllByText('Yours')).toHaveLength(1)
    expect(within(stream).getByText('Netflix').closest('li')).toHaveTextContent('Yours')
    expect(within(stream).getByText('Prime Video').closest('li')).not.toHaveTextContent('Yours')
  })

  it('labels every offer type', () => {
    const all = ['flatrate', 'free', 'ads', 'rent', 'buy'].map((offerType) =>
      create(ProviderOffersSchema, {
        offerType,
        providers: [create(ProviderSchema, { id: 1n, name: `via ${offerType}` })]
      })
    )
    render(<WhereToWatch offers={all} watchLink="" />)

    expect(screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)).toEqual([
      'Stream',
      'Free',
      'With ads',
      'Rent',
      'Buy'
    ])
  })

  it('credits JustWatch and links to the TMDB watch page', () => {
    render(<WhereToWatch offers={offers} watchLink="https://tmdb.example/watch" />)

    expect(screen.getByText(/Streaming availability data by JustWatch/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'See all on TMDB' })).toHaveAttribute(
      'href',
      'https://tmdb.example/watch'
    )
  })

  it('says so when there is nothing to stream, with no TMDB link', () => {
    render(<WhereToWatch offers={[]} watchLink="" />)

    expect(screen.getByText('Not available to stream in Belgium')).toBeInTheDocument()
    expect(screen.getByText(/by JustWatch/)).toBeInTheDocument()
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })
})
