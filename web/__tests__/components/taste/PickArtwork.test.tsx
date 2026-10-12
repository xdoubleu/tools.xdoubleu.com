import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import PickArtwork from '@/components/taste/PickArtwork'

describe('PickArtwork', () => {
  it('shows the cover, falling back to the initial when it fails', () => {
    const { container } = render(<PickArtwork url="https://img.example/h.jpg" title="hyperion" />)
    const img = container.querySelector('img')
    expect(img).toHaveAttribute('src', 'https://img.example/h.jpg')
    fireEvent.error(img!)
    expect(screen.getByText('H')).toBeInTheDocument()
    expect(container.querySelector('img')).toBeNull()
  })

  it('shows the initial without a cover', () => {
    const { container } = render(<PickArtwork url="" title="radiolab" />)
    expect(screen.getByText('R')).toHaveAttribute('aria-hidden', 'true')
    expect(container.querySelector('img')).toBeNull()
    expect(container.firstChild).toHaveStyle({ width: '64px', height: '64px' })
  })
})
