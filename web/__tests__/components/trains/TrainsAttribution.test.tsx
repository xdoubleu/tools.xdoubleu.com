import { render, screen } from '@testing-library/react'
import TrainsAttribution from '@/components/trains/TrainsAttribution'

describe('TrainsAttribution', () => {
  it('credits iRail with a link and NMBS/SNCB as the data origin', () => {
    render(<TrainsAttribution />)
    expect(screen.getByRole('link', { name: 'iRail' })).toHaveAttribute('href', 'https://irail.be')
    expect(screen.getByText(/originally published by NMBS\/SNCB/)).toBeInTheDocument()
  })
})
