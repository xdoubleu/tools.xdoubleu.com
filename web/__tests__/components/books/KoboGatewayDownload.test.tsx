import React from 'react'
import { render, screen } from '@testing-library/react'

import KoboGatewayDownload from '@/components/books/KoboGatewayDownload'

function setUserAgent(value: string) {
  Object.defineProperty(window.navigator, 'userAgent', {
    value,
    writable: true,
    configurable: true
  })
}

describe('KoboGatewayDownload', () => {
  it('renders the download button and Gatekeeper instructions on macOS', () => {
    setUserAgent('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605.1.15')

    render(<KoboGatewayDownload />)

    expect(screen.getByTestId('kobo-gateway-download')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /download kobo gateway/i })).toHaveAttribute(
      'href',
      '/downloads/kobo-gateway.dmg'
    )
    expect(screen.getByText(/open anyway/i)).toBeInTheDocument()
  })

  it('shows a non-Mac note on other platforms', () => {
    setUserAgent('Mozilla/5.0 (Windows NT 10.0; Win64; x64)')

    render(<KoboGatewayDownload />)

    expect(screen.queryByTestId('kobo-gateway-download')).not.toBeInTheDocument()
    expect(screen.getByTestId('kobo-gateway-non-mac')).toBeInTheDocument()
  })
})
