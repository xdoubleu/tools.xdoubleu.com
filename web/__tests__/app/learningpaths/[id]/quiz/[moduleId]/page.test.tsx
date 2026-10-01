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

jest.mock('@/app/learningpaths/[id]/quiz/[moduleId]/QuizClient', () => ({
  __esModule: true,
  default: ({ id, moduleId }: { id: string; moduleId: string }) => (
    <div data-testid="quiz-client">
      {id}/{moduleId}
    </div>
  )
}))

import ModuleQuizPage from '@/app/learningpaths/[id]/quiz/[moduleId]/page'

describe('ModuleQuizPage', () => {
  it('passes the path and module ids to QuizClient', async () => {
    fetchOrNull.mockResolvedValue({ learningPath: { id: 'lp1' } })
    const params = Promise.resolve({ id: 'lp1', moduleId: 'm1' })
    const { getByTestId } = render(await ModuleQuizPage({ params }))
    expect(getByTestId('quiz-client')).toHaveTextContent('lp1/m1')
  })

  it('renders when the server fetch returns null', async () => {
    fetchOrNull.mockResolvedValue(null)
    const params = Promise.resolve({ id: 'lp1', moduleId: 'm1' })
    const { getByTestId } = render(await ModuleQuizPage({ params }))
    expect(getByTestId('quiz-client')).toBeInTheDocument()
  })
})
