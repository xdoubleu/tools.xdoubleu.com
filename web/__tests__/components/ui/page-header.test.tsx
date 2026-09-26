import { render, screen } from '@testing-library/react'
import { PageHeader } from '@/components/ui/page-header'

describe('PageHeader', () => {
  it('renders the title as the page h1', () => {
    render(<PageHeader title="Recipes" />)
    expect(screen.getByRole('heading', { level: 1, name: 'Recipes' })).toBeInTheDocument()
  })

  it('renders breadcrumb, description and actions when given', () => {
    render(
      <PageHeader
        title="Edit"
        breadcrumb={[{ label: 'Recipes', href: '/recipes' }, { label: 'Edit' }]}
        description="Change the recipe"
        actions={<button>Save</button>}
      />
    )
    expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toBeInTheDocument()
    expect(screen.getByText('Change the recipe')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument()
  })

  it('omits the optional parts', () => {
    render(<PageHeader title="Plain" />)
    expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })
})
