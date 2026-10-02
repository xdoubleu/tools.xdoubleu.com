import { render } from '@testing-library/react'
import { usePathname } from 'next/navigation'
import ServiceWorkerRegistrar from '@/components/offline/ServiceWorkerRegistrar'
import { registerServiceWorker, savePageForOffline } from '@/lib/offline/session'

jest.mock('next/navigation', () => ({ usePathname: jest.fn() }))
jest.mock('@/lib/offline/session', () => ({
  registerServiceWorker: jest.fn(),
  savePageForOffline: jest.fn()
}))

describe('ServiceWorkerRegistrar', () => {
  it('registers once and saves each visited page', () => {
    jest.mocked(usePathname).mockReturnValue('/feeds')
    const { rerender } = render(<ServiceWorkerRegistrar />)

    jest.mocked(usePathname).mockReturnValue('/recipes')
    rerender(<ServiceWorkerRegistrar />)

    expect(registerServiceWorker).toHaveBeenCalledTimes(1)
    expect(savePageForOffline).toHaveBeenCalledTimes(2)
    expect(savePageForOffline).toHaveBeenLastCalledWith(window.location.href)
  })
})
