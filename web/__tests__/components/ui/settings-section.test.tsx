import { render, screen } from '@testing-library/react'
import { SettingsSection, SettingsSections } from '@/components/ui/settings-section'

describe('SettingsSection', () => {
  it('renders a section heading, description and content', () => {
    render(
      <SettingsSections>
        <SettingsSection
          title="Privacy"
          description="What we collect."
          actions={<button>Edit</button>}
        >
          <p>body</p>
        </SettingsSection>
      </SettingsSections>
    )
    expect(screen.getByRole('heading', { level: 2, name: 'Privacy' })).toHaveClass('font-semibold')
    expect(screen.getByText('What we collect.')).toHaveClass('text-muted')
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument()
    expect(screen.getByText('body').closest('section')?.parentElement).toHaveClass('space-y-10')
  })

  it('skips the description when absent', () => {
    const { container } = render(
      <SettingsSection title="MCP">
        <p>body</p>
      </SettingsSection>
    )
    expect(container.querySelectorAll('p')).toHaveLength(1)
  })
})
