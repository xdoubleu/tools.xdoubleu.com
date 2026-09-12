const mockRedirect = jest.fn()

jest.mock('next/navigation', () => ({
  redirect: (url: string) => mockRedirect(url)
}))

import LearningPathsPage from '@/app/learningpaths/page'

describe('LearningPathsPage', () => {
  it('redirects to /learningpaths/list', () => {
    LearningPathsPage()
    expect(mockRedirect).toHaveBeenCalledWith('/learningpaths/list')
  })
})
