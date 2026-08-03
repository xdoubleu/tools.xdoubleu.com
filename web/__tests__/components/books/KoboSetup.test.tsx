import React from 'react'
import { render, screen } from '@testing-library/react'

const mockUseGatewayStatus = jest.fn()
jest.mock('@/hooks/useKoboGateway', () => ({
  useGatewayStatus: () => mockUseGatewayStatus()
}))

jest.mock('@/components/books/KoboGatewaySetup', () => ({
  __esModule: true,
  default: () => <div data-testid="mock-gateway-setup" />
}))

jest.mock('@/components/books/KoboGatewayDownload', () => ({
  __esModule: true,
  default: () => <div data-testid="mock-gateway-download" />
}))

import KoboSetup from '@/components/books/KoboSetup'

describe('KoboSetup', () => {
  it('renders the download card when no gateway is found', () => {
    mockUseGatewayStatus.mockReturnValue({ data: undefined })

    render(<KoboSetup />)

    expect(screen.getByTestId('mock-gateway-download')).toBeInTheDocument()
    expect(screen.queryByTestId('mock-gateway-setup')).not.toBeInTheDocument()
  })

  it('renders the gateway setup flow once the gateway responds', () => {
    mockUseGatewayStatus.mockReturnValue({
      data: { version: 1, release: 'abc', kobos: [] }
    })

    render(<KoboSetup />)

    expect(screen.getByTestId('mock-gateway-setup')).toBeInTheDocument()
    expect(screen.queryByTestId('mock-gateway-download')).not.toBeInTheDocument()
  })
})
