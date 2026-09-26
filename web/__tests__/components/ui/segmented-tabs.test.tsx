import { fireEvent, render, screen } from '@testing-library/react'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'

describe('SegmentedTabs', () => {
  const options = [
    { value: 'pages', label: 'Pages' },
    { value: 'percent', label: 'Percent' }
  ] as const

  it('marks the current option selected', () => {
    render(
      <SegmentedTabs aria-label="Mode" value="pages" onChange={jest.fn()} options={[...options]} />
    )
    expect(screen.getByRole('tablist', { name: 'Mode' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Pages' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: 'Percent' })).toHaveAttribute('aria-selected', 'false')
  })

  it('reports the chosen value', () => {
    const onChange = jest.fn()
    render(
      <SegmentedTabs aria-label="Mode" value="pages" onChange={onChange} options={[...options]} />
    )
    fireEvent.click(screen.getByRole('tab', { name: 'Percent' }))
    expect(onChange).toHaveBeenCalledWith('percent')
  })
})
