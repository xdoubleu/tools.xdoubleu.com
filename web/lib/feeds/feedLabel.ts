import type { Feed } from '@/lib/gen/feeds/v1/feeds_pb'

// feedLabel names a feed: its title, else its URL; email feeds have neither.
export function feedLabel(feed: Pick<Feed, 'title' | 'url'>): string {
  return feed.title || feed.url || 'Email newsletter'
}
