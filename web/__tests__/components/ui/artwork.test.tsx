import { fireEvent, render, screen } from '@testing-library/react'
import { Artwork } from '@/components/ui/artwork'

describe('Artwork', () => {
  it.each([
    ['sm', '40px', '60px'],
    ['md', '48px', '72px'],
    ['lg', '128px', '192px'],
    ['square', '64px', '64px']
  ] as const)('reserves a %s box whether or not the image loads', (size, width, height) => {
    const { container } = render(<Artwork src="" title="Dune" size={size} />)
    expect(container.firstElementChild).toHaveStyle({ width, height, minWidth: width })
  })

  it('shows the image, decorative by default', () => {
    render(<Artwork src="https://img.example/a.jpg" title="history" size="square" />)
    const img = screen.getByRole('presentation')
    expect(img).toHaveAttribute('src', 'https://img.example/a.jpg')
    expect(img).toHaveAttribute('alt', '')
    expect(img).toHaveAttribute('width', '64')
    expect(img).toHaveAttribute('height', '64')
  })

  it('takes alt text when the image is the only label', () => {
    render(
      <Artwork src="https://img.example/c.jpg" title="The Odyssey" alt="The Odyssey" size="sm" />
    )
    expect(screen.getByRole('img')).toHaveAttribute('alt', 'The Odyssey')
  })

  it('falls back to the initial without an image', () => {
    render(<Artwork src="" title="history" size="square" />)
    expect(screen.queryByRole('presentation')).not.toBeInTheDocument()
    expect(screen.getByText('H')).toHaveClass('text-sm', 'bg-surface', 'text-muted')
  })

  it('falls back to the initial when the image fails', () => {
    render(<Artwork src="https://img.example/a.jpg" title="history" size="square" />)
    fireEvent.error(screen.getByRole('presentation'))
    expect(screen.getByText('H')).toBeInTheDocument()
  })

  it.each([
    ['The Lord of the Rings', 'TL'],
    ['Dune', 'D'],
    ['  my book ', 'MB']
  ])('shows up to two initials for %j', (title, initials) => {
    render(<Artwork src="" title={title} initials={2} size="sm" />)
    expect(screen.getByText(initials)).toHaveClass('text-[10px]')
  })

  it.each([
    ['md', 1, 'text-sm'],
    ['md', 2, 'text-xs'],
    ['lg', 1, 'text-2xl'],
    ['lg', 2, 'text-base']
  ] as const)('sizes a %s fallback of %i initial(s) as %s', (size, initials, text) => {
    render(<Artwork src="" title="ab cd" initials={initials} size={size} />)
    expect(screen.getByText(initials === 1 ? 'A' : 'AC')).toHaveClass(text)
  })

  it('merges a className onto the box', () => {
    const { container } = render(<Artwork src="" title="x" size="sm" className="mt-2" />)
    expect(container.firstElementChild).toHaveClass('mt-2', 'shrink-0')
  })
})
