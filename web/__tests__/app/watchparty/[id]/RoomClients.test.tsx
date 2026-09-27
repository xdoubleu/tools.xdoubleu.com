import { render, screen, fireEvent, act } from '@testing-library/react'
import ViewerClient from '@/app/watchparty/[id]/ViewerClient'
import PresenterClient from '@/app/watchparty/[id]/presenter/PresenterClient'

interface HookArgs {
  role: string
  onSharingChange?: (sharing: boolean) => void
  screenControls?: { current: { start: () => Promise<void>; stop: () => void } }
}

let lastArgs: HookArgs | undefined
let mockStatus = 'connected'
jest.mock('@/hooks/useWatchPartyRTC', () => ({
  useWatchPartyRTC: (args: HookArgs) => {
    lastArgs = args
    return {
      status: mockStatus,
      micEnabled: true,
      camEnabled: true,
      selfCamVisible: true,
      error: null,
      toggleMic: jest.fn(),
      toggleCam: jest.fn(),
      toggleSelfCam: jest.fn()
    }
  }
}))

jest.mock('@/lib/watchparty/roomUtils', () => ({
  attachDraggable: () => () => {}
}))

beforeEach(() => {
  lastArgs = undefined
  mockStatus = 'connected'
})

describe('ViewerClient', () => {
  it('joins as a viewer and renders the room', () => {
    render(<ViewerClient id="ROOM1" />)
    expect(lastArgs?.role).toBe('viewer')
    expect(screen.getByText('Room: ROOM1')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Share Screen' })).not.toBeInTheDocument()
  })
})

describe('PresenterClient', () => {
  it('starts and stops screen sharing', () => {
    render(<PresenterClient id="ROOM2" />)
    expect(lastArgs?.role).toBe('presenter')
    const start = jest.fn(() => Promise.resolve())
    const stop = jest.fn()
    lastArgs!.screenControls!.current = { start, stop }

    fireEvent.click(screen.getByRole('button', { name: 'Share Screen' }))
    expect(start).toHaveBeenCalled()

    act(() => lastArgs!.onSharingChange!(true))
    fireEvent.click(screen.getByRole('button', { name: 'Stop Sharing' }))
    expect(stop).toHaveBeenCalled()
  })

  it('disables sharing until connected', () => {
    mockStatus = 'connecting'
    render(<PresenterClient id="ROOM3" />)
    expect(screen.getByRole('button', { name: 'Share Screen' })).toBeDisabled()
  })
})
