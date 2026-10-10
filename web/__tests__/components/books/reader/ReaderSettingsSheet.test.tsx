import React from 'react'
import { render, screen } from '@testing-library/react'
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
})
