import { render, screen } from '@testing-library/react'

jest.mock('@/components/HomeClient', () => ({
  __esModule: true,
  default: () => <div data-testid="home-client" />
}))

import HomePage from '@/app/page'
import SignInPage from '@/app/auth/sign-in/page'

describe('HomePage', () => {
  it('renders the home client', () => {
    render(<HomePage />)
    expect(screen.getByTestId('home-client')).toBeInTheDocument()
  })
})

describe('SignInPage', () => {
  it('renders the home client in a narrow wrapper', () => {
    render(<SignInPage />)
    expect(screen.getByTestId('home-client')).toBeInTheDocument()
  })
})
