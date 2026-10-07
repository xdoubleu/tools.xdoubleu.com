import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('next/image', () => ({
  __esModule: true,
  default: ({ src, width }: { src: string; width: number }) => (
    // eslint-disable-next-line @next/next/no-img-element -- next/image stand-in
    <img src={src} alt="" width={width} data-testid="poster" />
  )
}))

import MoviePoster from '@/components/movies/MoviePoster'

describe('MoviePoster', () => {
  it.each([
    ['sm', '48px', '72px', 'text-sm', 'w92'],
    ['lg', '128px', '192px', 'text-2xl', 'w342']
  ] as const)('sizes %s posters and placeholders', (size, width, height, text, tmdb) => {
    const { container, rerender } = render(<MoviePoster posterPath="" title="dune" size={size} />)
    const box = container.firstElementChild
    expect(box).toHaveStyle({ width, height, minWidth: width })
    const initial = screen.getByText('D')
    expect(initial).toHaveClass(text, 'bg-surface', 'text-muted', 'font-semibold')

    rerender(<MoviePoster posterPath="/d.jpg" title="dune" size={size} />)
    expect(screen.getByTestId('poster')).toHaveAttribute(
      'src',
      `https://image.tmdb.org/t/p/${tmdb}/d.jpg`
    )
  })

  it('defaults to the small size', () => {
    const { container } = render(<MoviePoster posterPath="" title="x" />)
    expect(container.firstElementChild).toHaveStyle({ width: '48px' })
  })
})
