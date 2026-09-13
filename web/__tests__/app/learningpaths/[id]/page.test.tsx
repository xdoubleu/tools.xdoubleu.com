import React from 'react'
jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

const fetchOrNull = jest.fn()

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: (fn: () => Promise<unknown>) => fetchOrNull(fn)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

import { render } from '@testing-library/react'

jest.mock('@/app/learningpaths/[id]/PathClient', () => ({
  __esModule: true,
  default: ({ id }: { id: string }) => <div data-testid="path-client">{id}</div>
}))

import LearningPathPage from '@/app/learningpaths/[id]/page'

describe('LearningPathPage', () => {
  it('renders with server-fetched data', async () => {
    fetchOrNull.mockResolvedValue({ learningPath: { id: 'path-123' } })
    const params = Promise.resolve({ id: 'path-123' })
    const { getByTestId } = render(await LearningPathPage({ params }))
    expect(getByTestId('path-client')).toBeInTheDocument()
  })

  it('renders when the server fetch returns null', async () => {
    fetchOrNull.mockResolvedValue(null)
    const params = Promise.resolve({ id: 'path-123' })
    const { getByTestId } = render(await LearningPathPage({ params }))
    expect(getByTestId('path-client')).toBeInTheDocument()
  })

  it('passes the id from params to PathClient', async () => {
    fetchOrNull.mockResolvedValue(null)
    const params = Promise.resolve({ id: 'my-path-id' })
    const { getByText } = render(await LearningPathPage({ params }))
    expect(getByText('my-path-id')).toBeInTheDocument()
  })
})
