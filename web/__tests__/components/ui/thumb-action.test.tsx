import { fireEvent, render, screen } from '@testing-library/react'
import { ThumbAction, ThumbActionSpacer } from '@/components/ui/thumb-action'
import { EmptyState } from '@/components/ui/states'
import { focusById } from '@/lib/focusById'

describe('ThumbAction', () => {
  it('docks on phones and passes className through', () => {
    render(
      <ThumbAction className="extra" data-testid="bar">
        <button>Add</button>
      </ThumbAction>
    )
    expect(screen.getByTestId('bar')).toHaveClass('max-sm:fixed', 'extra')
    expect(screen.getByTestId('bar').className).toContain('max-sm:bg-glass')
    expect(screen.getByTestId('bar').className).toContain('max-sm:pb-[max(0.75rem')
    expect(screen.getByTestId('bar')).toHaveClass('max-sm:*:w-full')
    expect(screen.getByRole('button', { name: 'Add' })).toBeInTheDocument()
  })

  it('renders a hidden spacer', () => {
    const { container } = render(<ThumbActionSpacer />)
    expect(container.firstChild).toHaveAttribute('aria-hidden', 'true')
  })
})

describe('focusById', () => {
  it('focuses the target from an empty-state action', () => {
    render(
      <>
        <input id="target" aria-label="target" />
        <EmptyState action={<button onClick={() => focusById('target')}>Go</button>}>
          Empty
        </EmptyState>
      </>
    )
    fireEvent.click(screen.getByRole('button', { name: 'Go' }))
    expect(screen.getByLabelText('target')).toHaveFocus()
  })

  it('centers the target when scrolling it into view', () => {
    const scroll = vi.fn()
    render(<input id="t2" aria-label="t2" />)
    screen.getByLabelText('t2').scrollIntoView = scroll
    focusById('t2')
    expect(scroll).toHaveBeenCalledWith({ block: 'center' })
  })

  it('ignores a missing id', () => {
    expect(() => focusById('nope')).not.toThrow()
  })
})
