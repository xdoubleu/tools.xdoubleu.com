import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/books/v1/library_pb'
import BooksTableRow from '@/components/books/BooksTableRow'
import type { BookColumn } from '@/components/books/booksTableColumns'

const columns: BookColumn[] = [
  { key: 'title', label: 'Title', renderCell: (ub) => ub.book?.title ?? '' },
  { key: 'pages', label: 'Pages', renderCell: () => 'pages' }
]
const ctx = { knownShelves: [], knownTags: [] }

function userBook(description: string) {
  return create(UserBookSchema, {
    id: 'ub-1',
    book: create(BookSchema, { title: 'Dune', description })
  })
}

function renderRow(ub = userBook('A desert planet.')) {
  return render(
    <table>
      <tbody>
        <BooksTableRow userBook={ub} columns={columns} ctx={ctx} />
      </tbody>
    </table>
  )
}

describe('BooksTableRow', () => {
  it('renders a toggle cell ahead of the columns', () => {
    renderRow()
    const cells = screen.getAllByRole('cell')
    expect(cells).toHaveLength(3)
    expect(cells[0]).toContainElement(screen.getByRole('button'))
    expect(cells[1]).toHaveTextContent('Dune')
  })

  it('expands the description in a full-width row it controls', () => {
    const { container } = renderRow()
    const toggle = screen.getByRole('button', { name: 'Show description of Dune' })
    expect(toggle).toHaveTextContent('▸')
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(screen.getAllByRole('row')[0]).not.toHaveClass('border-b-0')
    expect(container.querySelector('#book-description-ub-1')).toBeNull()

    fireEvent.click(toggle)

    const hide = screen.getByRole('button', { name: 'Hide description of Dune' })
    expect(hide).toHaveTextContent('▾')
    expect(hide).toHaveAttribute('aria-expanded', 'true')
    expect(hide).toHaveAttribute('aria-controls', 'book-description-ub-1')
    const [bookRow, descriptionRow] = screen.getAllByRole('row')
    expect(bookRow).toHaveClass('border-b-0')
    expect(descriptionRow.querySelector('td')).toHaveAttribute('colspan', '3')
    expect(container.querySelector('#book-description-ub-1')).toHaveTextContent('A desert planet.')

    fireEvent.click(hide)
    expect(screen.getAllByRole('row')).toHaveLength(1)
  })

  it('collapses when the description is cleared while open', () => {
    const { rerender, container } = renderRow()
    fireEvent.click(screen.getByRole('button', { name: 'Show description of Dune' }))
    rerender(
      <table>
        <tbody>
          <BooksTableRow userBook={userBook('')} columns={columns} ctx={ctx} />
        </tbody>
      </table>
    )
    expect(screen.getAllByRole('row')).toHaveLength(1)
    expect(container.querySelector('#book-description-ub-1')).toBeNull()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('has no toggle without a description or without a book', () => {
    const { unmount } = renderRow(userBook(''))
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    unmount()

    renderRow(create(UserBookSchema, { id: 'ub-2' }))
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.getAllByRole('cell')).toHaveLength(3)
  })
})
