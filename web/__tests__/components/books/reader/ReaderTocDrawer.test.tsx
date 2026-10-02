import React from 'react'
import { fireEvent, render, screen, within } from '@testing-library/react'
import ReaderTocDrawer from '@/components/books/reader/ReaderTocDrawer'
import type { FoliateTocItem } from '@/lib/books/foliate'

const TOC: FoliateTocItem[] = [
  { label: 'Part One', subitems: [{ label: 'Chapter 1', href: 'c1.xhtml', subitems: [] }] },
  { label: 'Epilogue', href: 'end.xhtml' }
]

function renderDrawer(currentHref?: string) {
  const onSelect = jest.fn()
  render(
    <ReaderTocDrawer
      open
      onOpenChange={jest.fn()}
      toc={TOC}
      currentHref={currentHref}
      onSelect={onSelect}
    />
  )
  return { onSelect, nav: screen.getByRole('navigation', { name: 'Table of contents' }) }
}

describe('ReaderTocDrawer', () => {
  it('shows entries without a target as labels, not buttons', () => {
    const { nav } = renderDrawer()
    expect(within(nav).getByText('Part One').tagName).toBe('P')
    expect(within(nav).queryByRole('button', { name: 'Part One' })).not.toBeInTheDocument()
  })

  it('indents nested entries and skips empty sublists', () => {
    const { nav } = renderDrawer()
    const lists = nav.querySelectorAll('ul')
    expect(lists).toHaveLength(2)
    expect(lists[0]).not.toHaveClass('ml-4')
    expect(lists[1]).toHaveClass('ml-4')
  })

  it('selects an entry by its href', () => {
    const { nav, onSelect } = renderDrawer()
    fireEvent.click(within(nav).getByRole('button', { name: 'Chapter 1' }))
    expect(onSelect).toHaveBeenCalledWith('c1.xhtml')
  })

  it('highlights only the current entry', () => {
    const { nav } = renderDrawer('end.xhtml')
    const current = within(nav).getByRole('button', { name: 'Epilogue' })
    expect(current).toHaveAttribute('aria-current', 'location')
    expect(current).toHaveClass('text-accent')
    const other = within(nav).getByRole('button', { name: 'Chapter 1' })
    expect(other).not.toHaveAttribute('aria-current')
    expect(other).not.toHaveClass('text-accent')
  })
})
