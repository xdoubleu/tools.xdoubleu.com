import { render } from '@testing-library/react'

jest.mock('@/app/watchparty/[id]/presenter/PresenterClient', () => ({
  __esModule: true,
  default: ({ id }: { id: string }) => <div data-testid="presenter-client">{id}</div>
}))

import PresenterPage from '@/app/watchparty/[id]/presenter/page'

describe('PresenterPage', () => {
  it('renders without throwing', async () => {
    const params = Promise.resolve({ id: 'session-123' })
    const { getByTestId } = render(await PresenterPage({ params }))
    expect(getByTestId('presenter-client')).toBeInTheDocument()
  })

  it('passes the id from params to PresenterClient', async () => {
    const params = Promise.resolve({ id: 'my-session-id' })
    const { getByText } = render(await PresenterPage({ params }))
    expect(getByText('my-session-id')).toBeInTheDocument()
  })
})
