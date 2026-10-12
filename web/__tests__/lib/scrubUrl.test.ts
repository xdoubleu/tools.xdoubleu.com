import type { CaptureResult } from 'posthog-js'
import { scrubPostHogEvent, stripSensitiveParams } from '@/lib/scrubUrl'

describe('stripSensitiveParams', () => {
  it('redacts the reset token', () => {
    expect(stripSensitiveParams('https://x.test/auth/reset-password?token=abc&a=1')).toBe(
      'https://x.test/auth/reset-password?token=redacted&a=1'
    )
  })

  it('leaves other URLs untouched', () => {
    expect(stripSensitiveParams('https://x.test/books?page=2')).toBe('https://x.test/books?page=2')
    expect(stripSensitiveParams('not a url')).toBe('not a url')
  })
})

describe('scrubPostHogEvent', () => {
  it('redacts URL properties', () => {
    const event: CaptureResult = {
      uuid: 'u',
      event: '$pageview',
      properties: {
        $current_url: 'https://x.test/auth/reset-password?token=abc',
        $referrer: 'https://x.test/?code=1',
        other: 'https://x.test/?token=kept'
      }
    }

    const out = scrubPostHogEvent(event)

    expect(out?.properties.$current_url).toBe('https://x.test/auth/reset-password?token=redacted')
    expect(out?.properties.$referrer).toBe('https://x.test/?code=redacted')
    expect(out?.properties.other).toBe('https://x.test/?token=kept')
  })

  it('keeps element attribution on dead clicks', () => {
    const event: CaptureResult = {
      uuid: 'u',
      event: '$dead_click',
      properties: {
        $current_url: 'https://x.test/trains',
        $el_text: 'Live',
        $elements: [{ tag_name: 'button', $el_text: 'Live' }]
      }
    }

    const out = scrubPostHogEvent(event)

    expect(out?.properties.$el_text).toBe('Live')
    expect(out?.properties.$elements).toEqual([{ tag_name: 'button', $el_text: 'Live' }])
  })

  it('passes a dropped event through', () => {
    expect(scrubPostHogEvent(null)).toBeNull()
  })
})
