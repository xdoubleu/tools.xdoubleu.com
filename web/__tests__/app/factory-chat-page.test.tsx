import React from 'react'
import { render, screen } from '@testing-library/react'

vi.mock('@/components/factory/ChatClient', () => ({
  __esModule: true,
  default: () => <div data-testid="chat" />
}))

import FactoryChatPage from '@/app/factory/chat/page'

describe('FactoryChatPage', () => {
  it('renders the chat', () => {
    render(<FactoryChatPage />)
    expect(screen.getByTestId('chat')).toBeInTheDocument()
  })
})
