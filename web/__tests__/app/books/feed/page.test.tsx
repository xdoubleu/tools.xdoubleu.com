jest.mock('next/navigation', () => ({
  redirect: jest.fn()
}))

import { redirect } from 'next/navigation'
import ReadingFeedPage from '@/app/books/feed/page'

describe('ReadingFeedPage', () => {
  it('redirects to /feeds', () => {
    ReadingFeedPage()
    expect(redirect).toHaveBeenCalledWith('/feeds')
  })
})
