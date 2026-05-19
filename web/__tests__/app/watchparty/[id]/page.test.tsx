import { render } from '@testing-library/react'

jest.mock('@/app/watchparty/[id]/ViewerClient', () => ({
  __esModule: true,
  default: ({ id }: { id: string }) => <div data-testid="viewer-client">{id}</div>
}))

import ViewerPage from '@/app/watchparty/[id]/page'

describe('ViewerPage', () => {
  it('renders without throwing', async () => {
    const params = Promise.resolve({ id: 'session-123' })
    const { getByTestId } = render(await ViewerPage({ params }))
    expect(getByTestId('viewer-client')).toBeInTheDocument()
  })

  it('passes the id from params to ViewerClient', async () => {
    const params = Promise.resolve({ id: 'my-session-id' })
    const { getByText } = render(await ViewerPage({ params }))
    expect(getByText('my-session-id')).toBeInTheDocument()
  })
})
