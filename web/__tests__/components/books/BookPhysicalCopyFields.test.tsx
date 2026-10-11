import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { PhysicalChapterSchema } from '@/lib/gen/books/v1/library_pb'
import BookPhysicalCopyFields from '@/components/books/BookPhysicalCopyFields'
import type { PhysicalCopyDraft } from '@/lib/books/physicalCopy'

const chapters = [
  create(PhysicalChapterSchema, { title: 'One', position: { href: 'a', offset: 0 } }),
  create(PhysicalChapterSchema, { title: 'Two', position: { href: 'b', offset: 0 } })
]
const draft: PhysicalCopyDraft = {
  pageCount: 300,
  textStartPage: 1,
  anchors: [
    { page: 10, href: 'a', offset: 0 },
    { page: 90, href: 'b', offset: 0 }
  ]
}

function open(props: Partial<React.ComponentProps<typeof BookPhysicalCopyFields>> = {}) {
  const onChange = jest.fn()
  render(
    <BookPhysicalCopyFields
      draft={draft}
      chapters={chapters}
      hasPrintPages={false}
      onChange={onChange}
      {...props}
    />
  )
  fireEvent.click(screen.getByRole('button', { name: /Copy details/ }))
  return onChange
}

describe('BookPhysicalCopyFields', () => {
  it('edits only the anchor whose page changed', () => {
    const onChange = open()
    fireEvent.change(screen.getAllByLabelText('Page')[1]!, { target: { value: '95' } })
    expect(onChange).toHaveBeenCalledWith({
      ...draft,
      anchors: [draft.anchors[0], { page: 95, href: 'b', offset: 0 }]
    })
  })

  it('moves an anchor to another chapter', () => {
    const onChange = open()
    fireEvent.change(screen.getAllByLabelText('Chapter')[0]!, { target: { value: 'b#0' } })
    expect(onChange).toHaveBeenCalledWith({
      ...draft,
      anchors: [{ page: 10, href: 'b', offset: 0 }, draft.anchors[1]]
    })
  })

  it('removes only the chosen anchor', () => {
    const onChange = open()
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove chapter anchor' })[0]!)
    expect(onChange).toHaveBeenCalledWith({ ...draft, anchors: [draft.anchors[1]] })
  })

  it('offers no anchors without chapters', () => {
    open({ chapters: [], draft: { ...draft, anchors: [] } })
    expect(screen.queryByRole('button', { name: 'Add chapter anchor' })).not.toBeInTheDocument()
  })
})
