import { fireEvent, render, screen } from '@testing-library/react'
import { RemoveButton } from '@/components/ui/remove-button'

describe('RemoveButton', () => {
  it('is a labelled ghost × on the compact icon size', () => {
    const onClick = vi.fn()
    render(<RemoveButton aria-label="Remove Flour" onClick={onClick} />)
    const button = screen.getByRole('button', { name: 'Remove Flour' })
    expect(button).toHaveAttribute('type', 'button')
    expect(button).toHaveTextContent('×')
    expect(button).toHaveClass(
      'size-11',
      'sm:size-7',
      'hover:text-danger',
      'focus-visible:text-danger'
    )
    fireEvent.click(button)
    expect(onClick).toHaveBeenCalledTimes(1)
  })
})
