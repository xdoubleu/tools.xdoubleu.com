import { createRef } from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import WatchPartyRoom, { type WatchPartyRoomProps } from '@/components/watchparty/WatchPartyRoom'

const mockAttachDraggable = jest.fn()
jest.mock('@/lib/watchparty/roomUtils', () => ({
  attachDraggable: (el: HTMLElement) => mockAttachDraggable(el)
}))

function renderRoom(overrides: Partial<WatchPartyRoomProps> = {}) {
  const props: WatchPartyRoomProps = {
    id: 'ABC123',
    crumb: 'Room',
    status: 'connected',
    error: null,
    micEnabled: true,
    camEnabled: true,
    selfCamVisible: true,
    toggleMic: jest.fn(),
    toggleCam: jest.fn(),
    toggleSelfCam: jest.fn(),
    mainVideoRef: createRef<HTMLVideoElement>(),
    selfCamRef: createRef<HTMLVideoElement>(),
    remoteCamRef: createRef<HTMLVideoElement>(),
    ...overrides
  }
  return { props, ...render(<WatchPartyRoom {...props} />) }
}

beforeEach(() => {
  mockAttachDraggable.mockReset()
  mockAttachDraggable.mockReturnValue(jest.fn())
})

describe('WatchPartyRoom', () => {
  it('renders the breadcrumb, status, and room code', () => {
    renderRoom({ crumb: 'Presenter', status: 'connecting' })
    expect(screen.getByText('Presenter')).toBeInTheDocument()
    expect(screen.getByText('Connecting…')).toBeInTheDocument()
    expect(screen.getByText('Room: ABC123')).toBeInTheDocument()
  })

  it('makes both camera videos draggable and cleans up on unmount', () => {
    const cleanup = jest.fn()
    mockAttachDraggable.mockReturnValue(cleanup)
    const { unmount } = renderRoom()
    expect(mockAttachDraggable).toHaveBeenCalledTimes(2)
    unmount()
    expect(cleanup).toHaveBeenCalledTimes(2)
  })

  it('sizes the camera PiPs responsively', () => {
    renderRoom()
    const self = screen.getByLabelText('Your camera')
    expect(self).toHaveClass('w-24', 'sm:w-48', 'aspect-square')
    expect(self.style.width).toBe('')
  })

  it('shows the error in an alert', () => {
    renderRoom({ error: 'Camera blocked' })
    expect(screen.getByRole('alert')).toHaveTextContent('Camera blocked')
  })

  it('wires the media toggles', () => {
    const { props } = renderRoom()
    fireEvent.click(screen.getByRole('button', { name: 'Mute Mic' }))
    fireEvent.click(screen.getByRole('button', { name: 'Disable Cam' }))
    fireEvent.click(screen.getByRole('button', { name: 'Hide Self' }))
    expect(props.toggleMic).toHaveBeenCalled()
    expect(props.toggleCam).toHaveBeenCalled()
    expect(props.toggleSelfCam).toHaveBeenCalled()
  })

  it('labels the toggles for the off state', () => {
    renderRoom({ micEnabled: false, camEnabled: false, selfCamVisible: false })
    expect(screen.getByRole('button', { name: 'Unmute Mic' })).toHaveClass('bg-warn')
    expect(screen.getByRole('button', { name: 'Enable Cam' })).toHaveClass('bg-warn')
    expect(screen.getByRole('button', { name: 'Show Self' })).toHaveClass('text-muted')
  })

  it('renders extra controls', () => {
    renderRoom({ controls: <button type="button">Share Screen</button> })
    expect(screen.getByRole('button', { name: 'Share Screen' })).toBeInTheDocument()
  })
})
