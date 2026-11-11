import { fireEvent, render, screen } from '@testing-library/react'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'

describe('SegmentedTabs', () => {
  const options = [
    { value: 'pages', label: 'Pages' },
    { value: 'percent', label: 'Percent' }
  ] as const

  it('marks the current option selected', () => {
    render(
      <SegmentedTabs aria-label="Mode" value="pages" onChange={vi.fn()} options={[...options]} />
    )
    expect(screen.getByRole('tablist', { name: 'Mode' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Pages' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: 'Percent' })).toHaveAttribute('aria-selected', 'false')
    expect(screen.getByRole('tab', { name: 'Pages' })).not.toHaveAttribute('aria-pressed')
  })

  it('reports the chosen value', () => {
    const onChange = vi.fn()
    render(
      <SegmentedTabs aria-label="Mode" value="pages" onChange={onChange} options={[...options]} />
    )
    fireEvent.click(screen.getByRole('tab', { name: 'Percent' }))
    expect(onChange).toHaveBeenCalledWith('percent')
  })

  it('slides one indicator under the selected option', () => {
    const { container, rerender } = render(
      <SegmentedTabs aria-label="Mode" value="percent" onChange={vi.fn()} options={[...options]} />
    )
    const indicator = container.querySelector<HTMLElement>('[aria-hidden]')!
    expect(indicator.style.transform).toBe('translateX(100%)')
    expect(indicator.style.width).toBe('calc(50% - 2px)')
    rerender(
      <SegmentedTabs aria-label="Mode" value="pages" onChange={vi.fn()} options={[...options]} />
    )
    expect(indicator.style.transform).toBe('translateX(0%)')
  })

  it('shows no indicator when nothing is selected', () => {
    const { container } = render(
      <SegmentedTabs aria-label="Mode" value={null} onChange={vi.fn()} options={[...options]} />
    )
    expect(container.querySelector('[aria-hidden]')).toBeNull()
  })

  it('shares the control height', () => {
    render(
      <SegmentedTabs aria-label="Mode" value="pages" onChange={vi.fn()} options={[...options]} />
    )
    expect(screen.getByRole('tab', { name: 'Pages' })).toHaveClass('h-11', 'sm:h-9')
  })

  it('tells the selected tab from the others', () => {
    render(
      <SegmentedTabs aria-label="Mode" value="pages" onChange={vi.fn()} options={[...options]} />
    )
    expect(screen.getByRole('tab', { name: 'Pages' })).toHaveClass('text-fg')
    expect(screen.getByRole('tab', { name: 'Percent' })).toHaveClass('text-muted')
  })
})
