import { render, screen } from '@testing-library/react'
import AppGrid, { type AppLink, type AppSection } from '@/components/AppGrid'

const mockApps: AppLink[] = [
  { name: 'backlog', label: 'Backlog', href: '/backlog', description: 'Goals tracker' },
  { name: 'todos', label: 'Todos', href: '/todos', description: 'Task management' }
]

const mockSections: AppSection[] = [
  {
    title: 'Productivity',
    apps: [{ name: 'backlog', label: 'Backlog', href: '/backlog', description: 'Goals tracker' }]
  },
  {
    title: 'Account',
    apps: [{ name: 'settings', label: 'Settings', href: '/settings', description: 'Preferences' }]
  }
]

describe('AppGrid', () => {
  it('renders provided app links', () => {
    render(<AppGrid apps={mockApps} />)
    expect(screen.getByText('Backlog')).toBeInTheDocument()
    expect(screen.getByText('Todos')).toBeInTheDocument()
  })

  it('renders descriptions', () => {
    render(<AppGrid apps={mockApps} />)
    expect(screen.getByText('Goals tracker')).toBeInTheDocument()
  })

  it('renders links with correct hrefs', () => {
    render(<AppGrid apps={mockApps} />)
    expect(screen.getByRole('link', { name: /backlog/i })).toHaveAttribute('href', '/backlog')
  })

  it('renders nothing when apps is empty', () => {
    const { container } = render(<AppGrid apps={[]} />)
    expect(container.firstChild).toBeNull()
  })

  describe('sections', () => {
    it('renders section headings', () => {
      render(<AppGrid sections={mockSections} />)
      expect(screen.getByText('Productivity')).toBeInTheDocument()
      expect(screen.getByText('Account')).toBeInTheDocument()
    })

    it('renders apps within sections', () => {
      render(<AppGrid sections={mockSections} />)
      expect(screen.getByText('Backlog')).toBeInTheDocument()
      expect(screen.getByText('Settings')).toBeInTheDocument()
    })

    it('renders links with correct hrefs in sections', () => {
      render(<AppGrid sections={mockSections} />)
      expect(screen.getByRole('link', { name: /backlog/i })).toHaveAttribute('href', '/backlog')
      expect(screen.getByRole('link', { name: /settings/i })).toHaveAttribute('href', '/settings')
    })

    it('omits sections with no apps', () => {
      const sectionsWithEmpty: AppSection[] = [
        { title: 'Productivity', apps: [] },
        {
          title: 'Account',
          apps: [
            { name: 'settings', label: 'Settings', href: '/settings', description: 'Preferences' }
          ]
        }
      ]
      render(<AppGrid sections={sectionsWithEmpty} />)
      expect(screen.queryByText('Productivity')).not.toBeInTheDocument()
      expect(screen.getByText('Account')).toBeInTheDocument()
    })

    it('renders nothing when all sections are empty', () => {
      const { container } = render(<AppGrid sections={[{ title: 'Empty', apps: [] }]} />)
      expect(container.firstChild).toBeNull()
    })
  })
})
