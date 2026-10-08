import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'

jest.mock('next/image', () => ({
  __esModule: true,
  default: ({ src, onError }: { src: string; onError: () => void }) => (
    // eslint-disable-next-line @next/next/no-img-element -- next/image stand-in
    <img src={src} alt="" data-testid="artwork" onError={onError} />
  )
}))

import ShowArtwork from '@/components/podcasts/ShowArtwork'

describe('ShowArtwork', () => {
  it('shows the artwork', () => {
    const { container } = render(<ShowArtwork url="https://img.example/a.jpg" title="history" />)
    expect(screen.getByTestId('artwork')).toHaveAttribute('src', 'https://img.example/a.jpg')
    expect(container.firstElementChild).toHaveStyle({ width: '64px', height: '64px' })
  })

  it('falls back to the initial without a URL', () => {
    render(<ShowArtwork url="" title="history" />)
    expect(screen.getByText('H')).toBeInTheDocument()
    expect(screen.queryByTestId('artwork')).not.toBeInTheDocument()
  })

  it('falls back to the initial when the image fails', () => {
    render(<ShowArtwork url="https://img.example/a.jpg" title="history" />)
    fireEvent.error(screen.getByTestId('artwork'))
    expect(screen.getByText('H')).toBeInTheDocument()
  })
})
