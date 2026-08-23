import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ProviderConfigDialog from '@/components/monitoring/ProviderConfigDialog'

const mockFetchOptions = jest.fn()
const mockSetProviderConfig = jest.fn()

jest.mock('@/hooks/useMonitoring', () => ({
  useProviderOptions: () => mockFetchOptions,
  useSetProviderConfig: () => mockSetProviderConfig
}))

beforeEach(() => {
  jest.clearAllMocks()
})

describe('ProviderConfigDialog', () => {
  it('does not render content when closed', () => {
    render(<ProviderConfigDialog provider="github" open={false} onOpenChange={jest.fn()} />)
    expect(screen.queryByText(/Configure/)).not.toBeInTheDocument()
  })

  it('loads and picks a github repo, then saves', async () => {
    mockFetchOptions.mockResolvedValue({ repos: ['o/a', 'o/b'], sentryOrgs: [] })
    mockSetProviderConfig.mockResolvedValue(undefined)
    const onOpenChange = jest.fn()

    render(<ProviderConfigDialog provider="github" open={true} onOpenChange={onOpenChange} />)

    expect(mockFetchOptions).toHaveBeenCalledWith('github')
    await waitFor(() => expect(screen.getByText('o/a')).toBeInTheDocument())

    const saveButton = screen.getByRole('button', { name: 'Save' })
    expect(saveButton).toBeDisabled()

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'o/a' } })
    expect(saveButton).not.toBeDisabled()

    fireEvent.click(saveButton)

    await waitFor(() =>
      expect(mockSetProviderConfig).toHaveBeenCalledWith('github', {
        config: { case: 'github', value: { repo: 'o/a' } }
      })
    )
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
  })

  it('picks a sentry org then multiple projects, then saves', async () => {
    mockFetchOptions.mockImplementation((provider: string, org?: string) => {
      if (org) return Promise.resolve({ repos: [], sentryProjects: ['p1', 'p2'] })
      return Promise.resolve({ repos: [], sentryOrgs: ['org-a'] })
    })
    mockSetProviderConfig.mockResolvedValue(undefined)

    render(<ProviderConfigDialog provider="sentry" open={true} onOpenChange={jest.fn()} />)

    await waitFor(() => expect(screen.getByText('org-a')).toBeInTheDocument())

    const saveButton = screen.getByRole('button', { name: 'Save' })
    expect(saveButton).toBeDisabled()

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'org-a' } })
    expect(mockFetchOptions).toHaveBeenCalledWith('sentry', 'org-a')

    await waitFor(() => expect(screen.getByText('p1')).toBeInTheDocument())
    expect(saveButton).toBeDisabled()

    fireEvent.click(screen.getByLabelText('p1'))
    fireEvent.click(screen.getByLabelText('p2'))
    expect(saveButton).not.toBeDisabled()

    fireEvent.click(saveButton)

    await waitFor(() =>
      expect(mockSetProviderConfig).toHaveBeenCalledWith('sentry', {
        config: { case: 'sentry', value: { org: 'org-a', projects: ['p1', 'p2'] } }
      })
    )
  })

  it('shows an error when loading options fails', async () => {
    mockFetchOptions.mockRejectedValue(new Error('boom'))

    render(<ProviderConfigDialog provider="github" open={true} onOpenChange={jest.fn()} />)

    await waitFor(() => expect(screen.getByText('boom')).toBeInTheDocument())
  })

  it('calls onOpenChange(false) on cancel', async () => {
    mockFetchOptions.mockResolvedValue({ repos: [], sentryOrgs: [] })
    const onOpenChange = jest.fn()

    render(<ProviderConfigDialog provider="github" open={true} onOpenChange={onOpenChange} />)
    await waitFor(() => expect(mockFetchOptions).toHaveBeenCalled())

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
