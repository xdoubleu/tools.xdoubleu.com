import React from 'react'
import { render, screen } from '@testing-library/react'
import { PageContainer } from '@/components/ui/page-container'

describe('PageContainer', () => {
  it('renders children', () => {
    render(<PageContainer>Hello</PageContainer>)
    expect(screen.getByText('Hello')).toBeInTheDocument()
  })

  it('caps reading and collection pages at the max-w-7xl frame by default', () => {
    const { container } = render(<PageContainer>content</PageContainer>)
    expect(container.firstChild).toHaveClass('max-w-7xl')
  })

  it('applies no max-width for size="wide"', () => {
    const { container } = render(<PageContainer size="wide">content</PageContainer>)
    expect(container.innerHTML).not.toMatch(/max-w-/)
  })

  it.each([
    ['narrow', '[&>:not(header)]:max-w-xl'],
    ['form', '[&>:not(header)]:max-w-2xl']
  ] as const)(
    'keeps the frame for size="%s" and caps the content under the header',
    (size, cap) => {
      const { container } = render(<PageContainer size={size}>content</PageContainer>)
      expect(container.firstChild).toHaveClass('max-w-7xl', cap)
    }
  )

  it('merges additional className', () => {
    const { container } = render(<PageContainer className="p-6">content</PageContainer>)
    expect(container.firstChild).toHaveClass('p-6')
  })

  it.each(['default', 'wide', 'narrow', 'form'] as const)(
    'starts size="%s" at the left edge',
    (size) => {
      const { container } = render(<PageContainer size={size}>content</PageContainer>)
      expect(container.firstChild).toHaveClass('w-full')
      expect(container.firstChild).not.toHaveClass('mx-auto')
    }
  )

  it('centres a small column for size="centered"', () => {
    const { container } = render(<PageContainer size="centered">content</PageContainer>)
    expect(container.firstChild).toHaveClass('mx-auto', 'max-w-xl')
  })
})
