import { render, screen } from '@testing-library/react'
import { PageHeader, PageHeaderLink, PageHeaderSettingsLink } from '@/components/ui/page-header'

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

describe('PageHeaderLink', () => {
  it('renders a ghost link with its icon', () => {
    render(
      <PageHeaderLink href="/feeds/stats" icon={<svg data-testid="icon" aria-hidden="true" />}>
        Stats
      </PageHeaderLink>
    )
    const link = screen.getByRole('link', { name: 'Stats' })
    expect(link).toHaveAttribute('href', '/feeds/stats')
    expect(link).toHaveClass('hover:bg-hover')
    expect(link).not.toHaveClass('border')
    expect(screen.getByTestId('icon')).toBeInTheDocument()
  })
})

describe('PageHeaderSettingsLink', () => {
  it('links to the settings page with the gear icon', () => {
    render(<PageHeaderSettingsLink href="/books/settings" />)
    const link = screen.getByRole('link', { name: 'Settings' })
    expect(link).toHaveAttribute('href', '/books/settings')
    expect(link.querySelector('svg')).toBeInTheDocument()
  })
})
