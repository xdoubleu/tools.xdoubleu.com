import { isSharedDashboard } from '@/lib/signIn'

const BOOK_READER = /^\/books\/[^/]+\/read(\/|$)/
const PRESENTER = /^\/watchparty\/[^/]+\/presenter(\/|$)/

/** Full-screen pages that cover the phone tab bar: the book reader, the watch party presenter, shared dashboards. */
export function hidesTabBar(pathname: string): boolean {
  return BOOK_READER.test(pathname) || PRESENTER.test(pathname) || isSharedDashboard(pathname)
}
