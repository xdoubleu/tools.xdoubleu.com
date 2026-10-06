import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import ReaderSettingsSheet from '@/components/books/reader/ReaderSettingsSheet'

function renderSheet(fontSize: number) {
  render(
    <ReaderSettingsSheet
      open
      onOpenChange={jest.fn()}
      theme="light"
      onThemeChange={jest.fn()}
      fontSize={fontSize}
      onFontSizeChange={jest.fn()}
      reflowable
    />
  )
}

describe('ReaderSettingsSheet', () => {
  it('stops shrinking text at the minimum', () => {
    renderSheet(80)
    expect(screen.getByRole('button', { name: 'Smaller text' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Larger text' })).toBeEnabled()
  })

  it('stops growing text at the maximum', () => {
    renderSheet(200)
    expect(screen.getByRole('button', { name: 'Larger text' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Smaller text' })).toBeEnabled()
  })

  it('allows both directions in between', () => {
    renderSheet(90)
    expect(screen.getByRole('button', { name: 'Smaller text' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Larger text' })).toBeEnabled()
  })

  it('offers no format switch without one', () => {
    renderSheet(100)
    expect(screen.queryByRole('tablist', { name: 'Format' })).not.toBeInTheDocument()
  })

  it('switches between the original and the converted KEPUB', () => {
    const onChange = jest.fn()
    render(
      <ReaderSettingsSheet
        open
        onOpenChange={jest.fn()}
        theme="light"
        onThemeChange={jest.fn()}
        fontSize={100}
        onFontSizeChange={jest.fn()}
        reflowable={false}
        format={{ value: 'original', original: 'pdf', onChange }}
      />
    )
    expect(screen.getByRole('tab', { name: 'Original (PDF)' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    fireEvent.click(screen.getByRole('tab', { name: 'Converted (KEPUB)' }))
    expect(onChange).toHaveBeenCalledWith('kepub')
  })
})
