import { render, screen } from '@testing-library/react'
import AppGrid, { type AppLink } from '@/components/AppGrid'

const mockApps: AppLink[] = [
  { name: 'backlog', label: 'Backlog', href: '/backlog', description: 'Goals tracker' },
  { name: 'todos', label: 'Todos', href: '/todos', description: 'Task management' }
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
})
