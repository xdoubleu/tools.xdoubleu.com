import { headerFallback } from '@/lib/games/artwork'

describe('headerFallback', () => {
  it('maps a capsule URL to the same app header', () => {
    expect(
      headerFallback(
        'https://shared.cloudflare.steamstatic.com/store_item_assets/steam/apps/10/capsule_616x353.jpg'
      )
    ).toBe('https://shared.cloudflare.steamstatic.com/steam/apps/10/header.jpg')
  })

  it('has no fallback for other URLs', () => {
    expect(headerFallback('https://example.com/own.jpg')).toBe('')
  })
})
