import { fireEvent, render, screen } from '@testing-library/react'
import { DateInput } from '@/components/ui/date-input'

function getTextInput(): HTMLInputElement {
  return screen.getByPlaceholderText('dd/mm/yyyy')
}

function getNativeInput(container: HTMLElement): HTMLInputElement {
  const input = container.querySelector('input[type="date"]')
  if (!(input instanceof HTMLInputElement)) throw new Error('native date input not found')
  return input
}

describe('DateInput', () => {
  it('renders the value as dd/MM/yyyy', () => {
    render(<DateInput value="2026-01-15" onChange={jest.fn()} />)
    expect(getTextInput().value).toBe('15/01/2026')
  })

  it('renders an empty value as an empty field', () => {
    render(<DateInput value="" onChange={jest.fn()} />)
    expect(getTextInput().value).toBe('')
  })

  it('emits ISO when a full valid date is typed', () => {
    const onChange = jest.fn()
    render(<DateInput value="" onChange={onChange} />)
    fireEvent.change(getTextInput(), { target: { value: '15/01/2026' } })
    expect(onChange).toHaveBeenCalledWith('2026-01-15')
  })

  it('emits nothing while typing a partial date', () => {
    const onChange = jest.fn()
    render(<DateInput value="" onChange={onChange} />)
    fireEvent.change(getTextInput(), { target: { value: '15/01' } })
    expect(onChange).not.toHaveBeenCalled()
  })

  it('zero-pads single-digit day and month', () => {
    const onChange = jest.fn()
    render(<DateInput value="" onChange={onChange} />)
    fireEvent.change(getTextInput(), { target: { value: '5/1/2026' } })
    expect(onChange).toHaveBeenCalledWith('2026-01-05')
  })

  it('rejects impossible dates', () => {
    const onChange = jest.fn()
    render(<DateInput value="" onChange={onChange} />)
    fireEvent.change(getTextInput(), { target: { value: '31/02/2026' } })
    fireEvent.change(getTextInput(), { target: { value: '29/02/2023' } })
    expect(onChange).not.toHaveBeenCalled()
  })

  it('accepts 29 February in a leap year', () => {
    const onChange = jest.fn()
    render(<DateInput value="" onChange={onChange} />)
    fireEvent.change(getTextInput(), { target: { value: '29/02/2024' } })
    expect(onChange).toHaveBeenCalledWith('2024-02-29')
  })

  it('reverts invalid text to the last valid value on blur and calls onBlur', () => {
    const onBlur = jest.fn()
    render(<DateInput value="2026-01-15" onChange={jest.fn()} onBlur={onBlur} />)
    fireEvent.change(getTextInput(), { target: { value: '99/99/9999' } })
    fireEvent.blur(getTextInput())
    expect(getTextInput().value).toBe('15/01/2026')
    expect(onBlur).toHaveBeenCalled()
  })

  it('normalizes unpadded valid text on blur', () => {
    render(<DateInput value="" onChange={jest.fn()} />)
    fireEvent.change(getTextInput(), { target: { value: '5/1/2026' } })
    fireEvent.blur(getTextInput())
    expect(getTextInput().value).toBe('05/01/2026')
  })

  it('emits an empty string when the field is cleared', () => {
    const onChange = jest.fn()
    render(<DateInput value="2026-01-15" onChange={onChange} />)
    fireEvent.change(getTextInput(), { target: { value: '' } })
    expect(onChange).toHaveBeenCalledWith('')
  })

  it('emits picker selections from the native date input', () => {
    const onChange = jest.fn()
    const { container } = render(<DateInput value="" onChange={onChange} />)
    fireEvent.change(getNativeInput(container), { target: { value: '2026-03-01' } })
    expect(onChange).toHaveBeenCalledWith('2026-03-01')
  })

  it('does not throw when the picker button is clicked without showPicker support', () => {
    const { container } = render(<DateInput value="" onChange={jest.fn()} />)
    expect(() => fireEvent.click(getNativeInput(container))).not.toThrow()
  })

  it('updates the display when the value prop changes externally', () => {
    const { rerender } = render(<DateInput value="2026-01-15" onChange={jest.fn()} />)
    rerender(<DateInput value="2026-03-01" onChange={jest.fn()} />)
    expect(getTextInput().value).toBe('01/03/2026')
  })

  it('puts the id on the text input so labels keep working', () => {
    render(<DateInput id="from" value="" onChange={jest.fn()} />)
    expect(getTextInput().id).toBe('from')
  })
})
