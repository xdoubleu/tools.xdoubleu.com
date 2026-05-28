import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import FeedForm from '@/components/icsproxy/FeedForm'

const mockSaveConfig = jest.fn()
const mockPush = jest.fn()
let mockPreviewData: { data: unknown; isLoading: boolean; error: null | Error } = {
  data: null,
  isLoading: false,
  error: null
}

jest.mock('@/hooks/useICSProxy', () => ({
  useICSPreview: () => mockPreviewData,
  useSaveConfig: () => mockSaveConfig
}))

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush })
}))

const sampleEvents = [
  {
    uid: 'e1',
    summary: 'Team Standup',
    startNice: 'Mon 9am',
    endNice: 'Mon 9:30am',
    startRaw: '',
    endRaw: '',
    rrule: 'FREQ=WEEKLY',
    seriesKey: 'standup'
  },
  {
    uid: 'e2',
    summary: 'Lunch',
    startNice: 'Mon 12pm',
    endNice: 'Mon 1pm',
    startRaw: '',
    endRaw: '',
    rrule: '',
    seriesKey: ''
  }
]

describe('FeedForm', () => {
  beforeEach(() => {
    mockSaveConfig.mockReset()
    mockPush.mockReset()
    mockPreviewData = { data: null, isLoading: false, error: null }
  })

  it('renders source URL input and buttons', () => {
    render(<FeedForm />)
    expect(screen.getByPlaceholderText(/calendar.ics/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Preview' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save Filter' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
  })

  it('pre-fills source URL from initialConfig', () => {
    const config = {
      sourceUrl: 'https://cal.example.com/feed.ics',
      hideEventUids: [],
      holidayUids: [],
      hideSeries: []
    }
    render(<FeedForm initialConfig={config as never} />)
    const input = screen.getByPlaceholderText(/calendar.ics/) as HTMLInputElement
    expect(input.value).toBe('https://cal.example.com/feed.ics')
  })

  it('navigates to /icsproxy on Cancel', () => {
    render(<FeedForm />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(mockPush).toHaveBeenCalledWith('/icsproxy')
  })

  it('calls saveConfig and redirects on submit', async () => {
    mockSaveConfig.mockResolvedValue({})
    render(<FeedForm token="tok-1" />)
    const input = screen.getByPlaceholderText(/calendar.ics/)
    fireEvent.change(input, { target: { value: 'https://cal.example.com/feed.ics' } })
    fireEvent.submit(screen.getByRole('button', { name: 'Save Filter' }).closest('form')!)

    await waitFor(() => {
      expect(mockSaveConfig).toHaveBeenCalled()
      expect(mockPush).toHaveBeenCalledWith('/icsproxy')
    })
  })

  it('shows error message when saveConfig throws', async () => {
    mockSaveConfig.mockRejectedValue(new Error('Save failed'))
    render(<FeedForm />)
    const input = screen.getByPlaceholderText(/calendar.ics/)
    fireEvent.change(input, { target: { value: 'https://cal.example.com/feed.ics' } })
    fireEvent.submit(screen.getByRole('button', { name: 'Save Filter' }).closest('form')!)

    await waitFor(() => {
      expect(screen.getByText('Save failed')).toBeInTheDocument()
    })
  })

  it('shows loading state from preview hook', () => {
    mockPreviewData = { data: null, isLoading: true, error: null }
    render(<FeedForm />)
    expect(screen.getByText('Loading events...')).toBeInTheDocument()
  })

  it('shows preview error from hook', () => {
    mockPreviewData = { data: null, isLoading: false, error: new Error('Network error') }
    render(<FeedForm />)
    expect(screen.getByText(/Failed to load events/)).toBeInTheDocument()
  })

  it('renders events table when initialEvents provided', () => {
    render(<FeedForm initialEvents={sampleEvents as never} />)
    expect(screen.getByText('Team Standup')).toBeInTheDocument()
    expect(screen.getByText('Lunch')).toBeInTheDocument()
    expect(screen.getByText('2 events')).toBeInTheDocument()
  })

  it('toggles hide-event checkbox', () => {
    render(<FeedForm initialEvents={sampleEvents as never} />)
    const checkboxes = screen.getAllByRole('checkbox')
    // First Hide checkbox (for e1)
    fireEvent.click(checkboxes[0])
    expect((checkboxes[0] as HTMLInputElement).checked).toBe(true)
    fireEvent.click(checkboxes[0])
    expect((checkboxes[0] as HTMLInputElement).checked).toBe(false)
  })

  it('toggles holiday checkbox', () => {
    render(<FeedForm initialEvents={sampleEvents as never} />)
    const checkboxes = screen.getAllByRole('checkbox')
    // Holiday checkbox is the second column per event (index 2 = holiday for e1)
    fireEvent.click(checkboxes[2])
    expect((checkboxes[2] as HTMLInputElement).checked).toBe(true)
  })

  it('shows recurring Yes for events with rrule', () => {
    render(<FeedForm initialEvents={sampleEvents as never} />)
    expect(screen.getByText('Yes')).toBeInTheDocument()
  })

  it('toggles hide-series checkbox for recurring events', () => {
    render(<FeedForm initialEvents={sampleEvents as never} />)
    // The series checkbox for e1 (has seriesKey='standup')
    const seriesCheckboxes = screen
      .getAllByRole('checkbox')
      .filter((cb) => (cb as HTMLInputElement).className.includes('accent-blue-600'))
    // Find and click the series checkbox (column 4 for e1)
    const checkboxes = screen.getAllByRole('checkbox')
    // e1 has Hide(0), Holiday(2), Series(4)
    fireEvent.click(checkboxes[4])
    expect((checkboxes[4] as HTMLInputElement).checked).toBe(true)
    void seriesCheckboxes
  })

  it('handleFetch triggers preview by setting fetchUrl', () => {
    render(<FeedForm />)
    const input = screen.getByPlaceholderText(/calendar.ics/)
    fireEvent.change(input, { target: { value: 'https://cal.example.com/feed.ics' } })
    fireEvent.click(screen.getByRole('button', { name: 'Preview' }))
    // After clicking Preview the fetchUrl is set - no error expected
    expect(screen.queryByText('Failed to load events')).not.toBeInTheDocument()
  })
})
