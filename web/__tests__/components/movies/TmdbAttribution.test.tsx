import { render, screen } from '@testing-library/react'
import TmdbAttribution from '@/components/movies/TmdbAttribution'

describe('TmdbAttribution', () => {
  it('shows the TMDB logo linking to TMDB and the required notice under Credits', () => {
    render(<TmdbAttribution />)
    expect(screen.getByRole('heading', { name: 'Credits' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'TMDB' })).toHaveAttribute(
      'href',
      'https://www.themoviedb.org/'
    )
    expect(
      screen.getByText(
        /This application uses TMDB and the TMDB APIs but is not endorsed, certified, or otherwise approved by TMDB\./
      )
    ).toBeInTheDocument()
  })
})
