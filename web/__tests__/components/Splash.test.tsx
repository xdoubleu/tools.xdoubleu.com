import { render, screen } from '@testing-library/react'
import Splash from '@/components/Splash'

describe('Splash', () => {
  it('renders the loading indicator', () => {
    render(<Splash />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })
})
