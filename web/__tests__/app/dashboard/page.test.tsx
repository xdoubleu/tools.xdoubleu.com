import { render, screen } from '@testing-library/react'
import Page from '@/app/dashboard/page'

describe('DashboardsPage', () => {
  it('renders links to the Games and Reading dashboards', () => {
    render(<Page />)

    expect(screen.getByRole('heading', { name: 'Dashboards' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Games/ })).toHaveAttribute('href', '/dashboard/games')
    expect(screen.getByRole('link', { name: /Reading/ })).toHaveAttribute(
      'href',
      '/dashboard/reading'
    )
  })
})
