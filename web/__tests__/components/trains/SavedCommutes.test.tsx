import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import SavedCommutes, { type CommutePair } from '@/components/trains/SavedCommutes'
import { SavedCommuteSchema, StationSchema, type SavedCommute } from '@/lib/gen/trains/v1/trains_pb'

const station = (stopId: string, name: string) =>
  create(StationSchema, { stopId, nameNl: name, nameFr: name, nameEn: name, displayName: name })

const commute = (over: Partial<SavedCommute> = {}): SavedCommute =>
  create(SavedCommuteSchema, {
    id: 'c1',
    label: 'Home to work',
    position: 0,
    origin: station('SA', 'Alpha'),
    destination: station('SB', 'Bravo'),
    ...over
  })

const pair: CommutePair = {
  originStopId: 'SA',
  originName: 'Alpha',
  destStopId: 'SB',
  destName: 'Bravo'
}

function setup(props: Partial<React.ComponentProps<typeof SavedCommutes>> = {}) {
  const onOpen = vi.fn()
  const onCreate = vi.fn().mockResolvedValue(undefined)
  const onRemove = vi.fn().mockResolvedValue(undefined)
  render(
    <SavedCommutes
      commutes={props.commutes ?? [commute()]}
      onOpen={props.onOpen ?? onOpen}
      onCreate={props.onCreate ?? onCreate}
      onRemove={props.onRemove ?? onRemove}
      currentPair={props.currentPair ?? null}
    />
  )
  return { onOpen, onCreate, onRemove }
}

describe('SavedCommutes', () => {
  it('opens a commute on tap', () => {
    const { onOpen } = setup()
    fireEvent.click(screen.getByRole('button', { name: 'Open Home to work' }))
    expect(onOpen).toHaveBeenCalledWith(
      expect.objectContaining({ originStopId: 'SA', destStopId: 'SB' })
    )
  })

  it('deletes a commute', async () => {
    const { onRemove } = setup()
    fireEvent.click(screen.getByRole('button', { name: 'Delete Home to work' }))
    await waitFor(() => expect(onRemove).toHaveBeenCalledWith('c1'))
  })

  it('shows an alert when deleting a commute fails', async () => {
    setup({ onRemove: vi.fn().mockRejectedValue(new Error('boom')) })
    fireEvent.click(screen.getByRole('button', { name: 'Delete Home to work' }))
    expect(await screen.findByRole('alert')).toHaveTextContent("Couldn't delete the commute.")
  })

  it('shows an alert when saving a commute fails', async () => {
    setup({ onCreate: vi.fn().mockRejectedValue(new Error('boom')), currentPair: pair, commutes: [] })
    fireEvent.click(screen.getByRole('button', { name: 'Save commute' }))
    expect(await screen.findByRole('alert')).toHaveTextContent("Couldn't save the commute.")
    expect(screen.getByRole('button', { name: 'Save commute' })).toBeEnabled()
  })

  it('shows an alert when saving the reverse fails', async () => {
    setup({ onCreate: vi.fn().mockRejectedValue(new Error('boom')) })
    fireEvent.click(screen.getByRole('button', { name: 'Save the reverse of Home to work' }))
    expect(await screen.findByRole('alert')).toHaveTextContent("Couldn't save the commute.")
  })

  it('offers the reverse direction only when it is not saved', async () => {
    const { onCreate } = setup()
    fireEvent.click(screen.getByRole('button', { name: 'Save the reverse of Home to work' }))
    await waitFor(() => expect(onCreate).toHaveBeenCalledWith('Bravo → Alpha', 'SB', 'SA'))
  })

  it('hides the reverse button when the reverse pair is already saved', () => {
    setup({
      commutes: [
        commute(),
        commute({
          id: 'c2',
          label: 'Work to home',
          origin: station('SB', 'Bravo'),
          destination: station('SA', 'Alpha')
        })
      ]
    })
    expect(
      screen.queryByRole('button', { name: 'Save the reverse of Home to work' })
    ).not.toBeInTheDocument()
  })

  it('saves the current pair with a custom label', async () => {
    const { onCreate } = setup({ commutes: [], currentPair: pair })
    fireEvent.change(screen.getByLabelText('Saved commute label'), {
      target: { value: 'Commute' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save commute' }))
    await waitFor(() => expect(onCreate).toHaveBeenCalledWith('Commute', 'SA', 'SB'))
  })

  it('falls back to a generated label when none is typed', async () => {
    const { onCreate } = setup({ commutes: [], currentPair: pair })
    fireEvent.click(screen.getByRole('button', { name: 'Save commute' }))
    await waitFor(() => expect(onCreate).toHaveBeenCalledWith('Alpha → Bravo', 'SA', 'SB'))
  })

  it('does not offer to save a pair that is already saved', () => {
    setup({ currentPair: pair })
    expect(screen.queryByRole('button', { name: 'Save commute' })).not.toBeInTheDocument()
  })

  it('renders nothing actionable when there are no commutes and no pair', () => {
    setup({ commutes: [], currentPair: null })
    expect(screen.queryByRole('listitem')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Save commute' })).not.toBeInTheDocument()
  })
})
