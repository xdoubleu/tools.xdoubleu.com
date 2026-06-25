import React from 'react'
import { render, screen } from '@testing-library/react'
import { PageContainer } from '@/components/ui/page-container'

describe('PageContainer', () => {
  it('renders children', () => {
    render(<PageContainer>Hello</PageContainer>)
    expect(screen.getByText('Hello')).toBeInTheDocument()
  })

  it('applies max-w-6xl by default', () => {
    const { container } = render(<PageContainer>content</PageContainer>)
    expect(container.firstChild).toHaveClass('max-w-6xl')
  })

  it('applies max-w-xl for size="narrow"', () => {
    const { container } = render(<PageContainer size="narrow">content</PageContainer>)
    expect(container.firstChild).toHaveClass('max-w-xl')
    expect(container.firstChild).not.toHaveClass('max-w-6xl')
  })

  it('merges additional className', () => {
    const { container } = render(<PageContainer className="p-6">content</PageContainer>)
    expect(container.firstChild).toHaveClass('p-6')
    expect(container.firstChild).toHaveClass('max-w-6xl')
  })

  it('always includes mx-auto and w-full', () => {
    const { container } = render(<PageContainer>content</PageContainer>)
    expect(container.firstChild).toHaveClass('mx-auto')
    expect(container.firstChild).toHaveClass('w-full')
  })
})
