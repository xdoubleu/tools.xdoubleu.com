import { fireEvent, render, screen } from '@testing-library/react'
import { DetailHeader } from '@/components/ui/detail-header'

const breadcrumb = [{ label: 'Movies', href: '/movies' }, { label: 'Dark' }]

describe('DetailHeader', () => {
  it('renders title, subtitle, facts, status and actions', () => {
    render(
      <DetailHeader
        breadcrumb={breadcrumb}
        title="Dark"
        subtitle="Original"
        meta="Series · 3 seasons"
        status={<p>status control</p>}
        favourite={<button>Fav</button>}
        actions={<button>Edit</button>}
        artwork={<div>art</div>}
      />
    )
    expect(screen.getByRole('heading', { name: 'Dark' })).toBeInTheDocument()
    expect(screen.getByText('Original')).toBeInTheDocument()
    expect(screen.getByText('Series · 3 seasons')).toBeInTheDocument()
    expect(screen.getByText('status control')).toBeInTheDocument()
    expect(screen.getByText('art')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'More actions' })).not.toBeInTheDocument()
  })

  it('runs a menu entry and closes the menu', () => {
    const onSelect = vi.fn()
    render(
      <DetailHeader
        breadcrumb={breadcrumb}
        title="Dark"
        menu={[{ label: 'Delete', destructive: true, onSelect }]}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'More actions' }))
    expect(screen.getByRole('menuitem', { name: 'Delete' })).toHaveClass('text-danger')
    fireEvent.click(screen.getByRole('menuitem', { name: 'Delete' }))
    expect(onSelect).toHaveBeenCalledOnce()
    expect(screen.queryByRole('menuitem', { name: 'Delete' })).not.toBeInTheDocument()
  })

  it('leaves out the facts line without facts', () => {
    const { container } = render(<DetailHeader breadcrumb={breadcrumb} title="Dark" />)
    expect(container.querySelector('p')).toBeNull()
  })

  it('does not mark a plain entry destructive', () => {
    render(
      <DetailHeader
        breadcrumb={breadcrumb}
        title="Dark"
        menu={[{ label: 'Copy', onSelect: vi.fn() }]}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'More actions' }))
    expect(screen.getByRole('menuitem', { name: 'Copy' })).not.toHaveClass('text-danger')
  })

  it('omits the menu for an empty list', () => {
    render(<DetailHeader breadcrumb={breadcrumb} title="Dark" menu={[]} />)
    expect(screen.queryByRole('button', { name: 'More actions' })).not.toBeInTheDocument()
  })
})
