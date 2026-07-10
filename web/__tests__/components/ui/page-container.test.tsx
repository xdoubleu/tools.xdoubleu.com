import React from 'react'
import { render, screen } from '@testing-library/react'
import { PageContainer } from '@/components/ui/page-container'

describe('PageContainer', () => {
  it('renders children', () => {
    render(<PageContainer>Hello</PageContainer>)
    expect(screen.getByText('Hello')).toBeInTheDocument()
  })

  it('applies no max-width by default', () => {
    const { container } = render(<PageContainer>content</PageContainer>)
    expect(container.innerHTML).not.toMatch(/max-w-/)
  })

  it('applies max-w-xl for size="narrow"', () => {
    const { container } = render(<PageContainer size="narrow">content</PageContainer>)
    expect(container.firstChild).toHaveClass('max-w-xl')
  })

  it('merges additional className', () => {
    const { container } = render(<PageContainer className="p-6">content</PageContainer>)
    expect(container.firstChild).toHaveClass('p-6')
  })

  it('always includes mx-auto and w-full', () => {
    const { container } = render(<PageContainer>content</PageContainer>)
    expect(container.firstChild).toHaveClass('mx-auto')
    expect(container.firstChild).toHaveClass('w-full')
  })
})
