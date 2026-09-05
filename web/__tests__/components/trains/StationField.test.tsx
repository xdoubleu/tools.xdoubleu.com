import { render, screen, fireEvent } from '@testing-library/react'
import StationField from '@/components/trains/StationField'

const mockUseStationSearch = jest.fn()
jest.mock('@/hooks/useTrains', () => ({
  useStationSearch: (query: string) => mockUseStationSearch(query)
}))

let capturedOnSelect: ((name: string) => void) | undefined
jest.mock('@/components/ui/combobox', () => ({
  Combobox: (props: {
    value: string
    onChange: (v: string) => void
    onSelect?: (name: string) => void
    'aria-label'?: string
  }) => {
    capturedOnSelect = props.onSelect
    return (
      <input
        aria-label={props['aria-label']}
        value={props.value}
        onChange={(e) => props.onChange(e.target.value)}
      />
    )
  }
}))

beforeEach(() => {
  mockUseStationSearch.mockReset()
  mockUseStationSearch.mockReturnValue({
    stations: [
      { stopId: 'SA', name: 'Alpha' },
      { stopId: 'SB', name: 'Bravo' }
    ]
  })
})

describe('StationField', () => {
  it('renders the label and current query', () => {
    render(
      <StationField
        label="From"
        query="Al"
        onQueryChange={jest.fn()}
        onSelectStation={jest.fn()}
        placeholder="Origin station"
      />
    )
    expect(screen.getByText('From')).toBeInTheDocument()
    expect(screen.getByLabelText('From')).toHaveValue('Al')
  })

  it('calls onQueryChange as the user types', () => {
    const onQueryChange = jest.fn()
    render(
      <StationField
        label="From"
        query=""
        onQueryChange={onQueryChange}
        onSelectStation={jest.fn()}
        placeholder="Origin station"
      />
    )
    fireEvent.change(screen.getByLabelText('From'), { target: { value: 'Al' } })
    expect(onQueryChange).toHaveBeenCalledWith('Al')
  })

  it('resolves a selected suggestion to its stop id', () => {
    const onSelectStation = jest.fn()
    render(
      <StationField
        label="From"
        query="Al"
        onQueryChange={jest.fn()}
        onSelectStation={onSelectStation}
        placeholder="Origin station"
      />
    )
    capturedOnSelect?.('Alpha')
    expect(onSelectStation).toHaveBeenCalledWith('SA', 'Alpha')
  })

  it('ignores a selection that does not match a known station', () => {
    const onSelectStation = jest.fn()
    render(
      <StationField
        label="From"
        query="Al"
        onQueryChange={jest.fn()}
        onSelectStation={onSelectStation}
        placeholder="Origin station"
      />
    )
    capturedOnSelect?.('Nowhere')
    expect(onSelectStation).not.toHaveBeenCalled()
  })
})
