import { render, screen, fireEvent, act } from '@testing-library/react'
import { LabelPicker } from '@/components/todos/LabelPicker'

const PRESETS = ['bug', 'feature', 'enhancement', 'documentation']

function setup(value: string[] = [], onChange = jest.fn()) {
  return render(
    <LabelPicker value={value} onChange={onChange} presets={PRESETS} />
  )
}

describe('LabelPicker', () => {
  it('renders the search input', () => {
    setup()
    expect(screen.getByRole('textbox', { name: /label search/i })).toBeInTheDocument()
  })

  it('shows no dropdown when input is not focused', () => {
    setup()
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('shows the dropdown on focus', () => {
    setup()
    fireEvent.focus(screen.getByRole('textbox', { name: /label search/i }))
    expect(screen.getByRole('listbox')).toBeInTheDocument()
  })

  it('shows all presets when dropdown opens with empty query', () => {
    setup()
    fireEvent.focus(screen.getByRole('textbox', { name: /label search/i }))
    const options = screen.getAllByRole('option')
    expect(options).toHaveLength(PRESETS.length)
  })

  it('filters presets as user types', () => {
    setup()
    const input = screen.getByRole('textbox', { name: /label search/i })
    fireEvent.focus(input)
    fireEvent.change(input, { target: { value: 'bug' } })
    expect(screen.getAllByRole('option')).toHaveLength(1)
    expect(screen.getByText('bug')).toBeInTheDocument()
  })

  it('calls onChange when a label is checked', () => {
    const onChange = jest.fn()
    setup([], onChange)
    fireEvent.focus(screen.getByRole('textbox', { name: /label search/i }))
    fireEvent.click(screen.getByRole('checkbox', { name: /bug/i }))
    expect(onChange).toHaveBeenCalledWith(['bug'])
  })

  it('calls onChange to remove a label when unchecked', () => {
    const onChange = jest.fn()
    setup(['bug'], onChange)
    fireEvent.focus(screen.getByRole('textbox', { name: /label search/i }))
    fireEvent.click(screen.getByRole('checkbox', { name: /bug/i }))
    expect(onChange).toHaveBeenCalledWith([])
  })

  it('renders selected label badges', () => {
    setup(['bug', 'feature'])
    expect(screen.getByText('bug')).toBeInTheDocument()
    expect(screen.getByText('feature')).toBeInTheDocument()
  })

  it('calls onChange to remove label via badge × button', () => {
    const onChange = jest.fn()
    setup(['bug'], onChange)
    fireEvent.click(screen.getByRole('button', { name: /remove bug/i }))
    expect(onChange).toHaveBeenCalledWith([])
  })

  it('hides the dropdown on blur after delay', async () => {
    jest.useFakeTimers()
    setup()
    const input = screen.getByRole('textbox', { name: /label search/i })
    fireEvent.focus(input)
    expect(screen.getByRole('listbox')).toBeInTheDocument()
    fireEvent.blur(input)
    act(() => { jest.advanceTimersByTime(200) })
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
    jest.useRealTimers()
  })
})
