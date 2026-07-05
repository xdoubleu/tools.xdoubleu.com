import React from 'react'

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

import { render } from '@testing-library/react'

jest.mock('@/app/todos/[id]/TaskClient', () => ({
  __esModule: true,
  default: ({ id }: { id: string }) => <div data-testid="task-client">{id}</div>
}))

import TaskPage from '@/app/todos/[id]/page'

describe('TaskPage', () => {
  it('renders without throwing', async () => {
    const params = Promise.resolve({ id: 'task-123' })
    const { getByTestId } = render(await TaskPage({ params }))
    expect(getByTestId('task-client')).toBeInTheDocument()
  })

  it('passes the id from params to TaskClient', async () => {
    const params = Promise.resolve({ id: 'my-task-id' })
    const { getByText } = render(await TaskPage({ params }))
    expect(getByText('my-task-id')).toBeInTheDocument()
  })
})
