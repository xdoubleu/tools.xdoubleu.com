import { render, screen } from '@testing-library/react'
import TrainsAttribution from '@/components/trains/TrainsAttribution'

describe('TrainsAttribution', () => {
  it('credits iRail with a link and NMBS/SNCB as the data origin', () => {
    render(<TrainsAttribution />)
    const link = screen.getByRole('link', { name: 'iRail' })
    expect(link).toHaveAttribute('href', 'https://irail.be')
    expect(screen.getByText(/originally published by NMBS\/SNCB/)).toBeInTheDocument()
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noreferrer')
  })

  it('makes the iRail link a full-size tap target', () => {
    render(<TrainsAttribution />)
    expect(screen.getByRole('link', { name: 'iRail' })).toHaveClass(
      'inline-block',
      'min-w-11',
      'py-4',
      '-my-4'
    )
  })
})
