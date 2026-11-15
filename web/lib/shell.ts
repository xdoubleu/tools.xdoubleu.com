import { isSharedDashboard } from '@/lib/signIn'

const BOOK_READER = /^\/books\/[^/]+\/read(\/|$)/

/** Full-screen pages that cover the phone tab bar: the book reader and shared dashboards. */
export function hidesTabBar(pathname: string): boolean {
  return BOOK_READER.test(pathname) || isSharedDashboard(pathname)
}
