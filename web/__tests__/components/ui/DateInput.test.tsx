import { fireEvent, render } from '@testing-library/react'
import { DateInput } from '@/components/ui/date-input'

function getInput(container: HTMLElement): HTMLInputElement {
  const input = container.querySelector('input')
  if (!(input instanceof HTMLInputElement)) throw new Error('date input not found')
  return input
}

describe('DateInput', () => {
  it('renders a native date input with the given value', () => {
    const { container } = render(<DateInput value="2026-01-15" onChange={jest.fn()} />)
    const input = getInput(container)
    expect(input.type).toBe('date')
    expect(input.value).toBe('2026-01-15')
  })

  it('renders an empty value as an empty field', () => {
    const { container } = render(<DateInput value="" onChange={jest.fn()} />)
    expect(getInput(container).value).toBe('')
  })

  it('emits the raw value on change', () => {
    const onChange = jest.fn()
    const { container } = render(<DateInput value="" onChange={onChange} />)
    fireEvent.change(getInput(container), { target: { value: '2026-01-15' } })
    expect(onChange).toHaveBeenCalledWith('2026-01-15')
  })

  it('emits an empty string when the field is cleared', () => {
    const onChange = jest.fn()
    const { container } = render(<DateInput value="2026-01-15" onChange={onChange} />)
    fireEvent.change(getInput(container), { target: { value: '' } })
    expect(onChange).toHaveBeenCalledWith('')
  })

  it('calls onBlur', () => {
    const onBlur = jest.fn()
    const { container } = render(
      <DateInput value="2026-01-15" onChange={jest.fn()} onBlur={onBlur} />
    )
    fireEvent.blur(getInput(container))
    expect(onBlur).toHaveBeenCalled()
  })

  it('updates when the value prop changes externally', () => {
    const { container, rerender } = render(<DateInput value="2026-01-15" onChange={jest.fn()} />)
    rerender(<DateInput value="2026-03-01" onChange={jest.fn()} />)
    expect(getInput(container).value).toBe('2026-03-01')
  })

  it('puts the id on the input so labels keep working', () => {
    const { container } = render(<DateInput id="from" value="" onChange={jest.fn()} />)
    expect(getInput(container).id).toBe('from')
  })

  it('puts the aria-label on the input', () => {
    const { container } = render(<DateInput value="" onChange={jest.fn()} aria-label="Date" />)
    expect(getInput(container).getAttribute('aria-label')).toBe('Date')
  })
})
