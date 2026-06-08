import { render } from '@testing-library/react'
import SettingsIcon from '@/components/SettingsIcon'

describe('SettingsIcon', () => {
  it('renders an aria-hidden svg with default size', () => {
    const { container } = render(<SettingsIcon />)
    const svg = container.querySelector('svg')
    expect(svg).toBeInTheDocument()
    expect(svg).toHaveAttribute('aria-hidden', 'true')
    expect(svg).toHaveAttribute('width', '14')
    expect(svg).toHaveAttribute('height', '14')
  })

  it('applies a custom size and className', () => {
    const { container } = render(<SettingsIcon size={20} className="text-accent" />)
    const svg = container.querySelector('svg')
    expect(svg).toHaveAttribute('width', '20')
    expect(svg).toHaveAttribute('height', '20')
    expect(svg).toHaveClass('text-accent')
  })
})
