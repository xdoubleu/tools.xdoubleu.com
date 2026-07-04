import {
  parseKoboConf,
  serializeKoboConf,
  patchApiEndpoint,
  revertApiEndpoint,
  getApiEndpoint,
  KOBO_DEFAULT_ENDPOINT
} from '@/lib/books/koboConf'

const SAMPLE_CONF = `[OneStoreServices]
api_endpoint=https://storeapi.kobo.com
affiliate=Kobo

[Version]
BuildVersion=4.37.21586
FirmwareVersion=4.37.21586`

describe('parseKoboConf', () => {
  it('parses sections and key=value pairs', () => {
    const conf = parseKoboConf(SAMPLE_CONF)
    expect(conf['OneStoreServices']['api_endpoint']).toBe('https://storeapi.kobo.com')
    expect(conf['OneStoreServices']['affiliate']).toBe('Kobo')
    expect(conf['Version']['BuildVersion']).toBe('4.37.21586')
  })

  it('handles CRLF line endings', () => {
    const crlf = '[OneStoreServices]\r\napi_endpoint=https://example.com\r\n'
    const conf = parseKoboConf(crlf)
    expect(conf['OneStoreServices']['api_endpoint']).toBe('https://example.com')
  })

  it('preserves values with = characters (e.g. URLs with query strings)', () => {
    const raw = '[S]\nkey=https://host/path?a=1&b=2'
    const conf = parseKoboConf(raw)
    expect(conf['S']['key']).toBe('https://host/path?a=1&b=2')
  })

  it('returns empty object for empty input', () => {
    expect(parseKoboConf('')).toEqual({})
  })

  it('ignores lines outside sections', () => {
    const raw = 'orphan=value\n[S]\nkey=val'
    const conf = parseKoboConf(raw)
    expect(conf['S']['key']).toBe('val')
    expect(conf['orphan']).toBeUndefined()
  })

  it('handles malformed lines without = gracefully', () => {
    const raw = '[S]\nnot-a-pair\nkey=val'
    const conf = parseKoboConf(raw)
    expect(conf['S']['key']).toBe('val')
    expect(conf['S']['not-a-pair']).toBeUndefined()
  })
})

describe('serializeKoboConf', () => {
  it('round-trips a parsed conf', () => {
    const conf = parseKoboConf(SAMPLE_CONF)
    const out = serializeKoboConf(conf)
    const reparsed = parseKoboConf(out)
    expect(reparsed['OneStoreServices']['api_endpoint']).toBe('https://storeapi.kobo.com')
    expect(reparsed['Version']['BuildVersion']).toBe('4.37.21586')
  })

  it('formats sections with headers and key=value pairs', () => {
    const conf = { S: { k: 'v' } }
    expect(serializeKoboConf(conf)).toBe('[S]\nk=v')
  })

  it('separates sections with a blank line', () => {
    const conf = { A: { k: '1' }, B: { k: '2' } }
    const out = serializeKoboConf(conf)
    expect(out).toContain('[A]\nk=1\n\n[B]\nk=2')
  })
})

describe('patchApiEndpoint', () => {
  it('sets api_endpoint in OneStoreServices', () => {
    const conf = parseKoboConf(SAMPLE_CONF)
    const { conf: patched } = patchApiEndpoint(conf, 'https://myserver/books/kobo/TOKEN')
    expect(getApiEndpoint(patched)).toBe('https://myserver/books/kobo/TOKEN')
  })

  it('returns the original api_endpoint', () => {
    const conf = parseKoboConf(SAMPLE_CONF)
    const { originalEndpoint } = patchApiEndpoint(conf, 'https://myserver/books/kobo/TOKEN')
    expect(originalEndpoint).toBe('https://storeapi.kobo.com')
  })

  it('preserves other keys in OneStoreServices', () => {
    const conf = parseKoboConf(SAMPLE_CONF)
    const { conf: patched } = patchApiEndpoint(conf, 'https://myserver/books/kobo/TOKEN')
    expect(patched['OneStoreServices']['affiliate']).toBe('Kobo')
  })

  it('preserves other sections', () => {
    const conf = parseKoboConf(SAMPLE_CONF)
    const { conf: patched } = patchApiEndpoint(conf, 'https://myserver/books/kobo/TOKEN')
    expect(patched['Version']['BuildVersion']).toBe('4.37.21586')
  })

  it('creates OneStoreServices section if missing', () => {
    const conf = { Version: { BuildVersion: '1.0' } }
    const { conf: patched, originalEndpoint } = patchApiEndpoint(conf, 'https://myserver/kobo/T')
    expect(getApiEndpoint(patched)).toBe('https://myserver/kobo/T')
    expect(originalEndpoint).toBe('')
  })

  it('is idempotent when called twice with the same URL', () => {
    const conf = parseKoboConf(SAMPLE_CONF)
    const url = 'https://myserver/books/kobo/TOKEN'
    const { conf: once, originalEndpoint: orig1 } = patchApiEndpoint(conf, url)
    const { conf: twice, originalEndpoint: orig2 } = patchApiEndpoint(once, url)
    expect(getApiEndpoint(twice)).toBe(url)
    expect(orig1).toBe('https://storeapi.kobo.com')
    expect(orig2).toBe(url)
  })
})

describe('KOBO_DEFAULT_ENDPOINT', () => {
  it('is the stock Kobo store URL', () => {
    expect(KOBO_DEFAULT_ENDPOINT).toBe('https://storeapi.kobo.com')
  })

  it('can be used with revertApiEndpoint to restore the device default', () => {
    const conf = parseKoboConf(SAMPLE_CONF)
    const { conf: patched } = patchApiEndpoint(conf, 'https://myserver/books/kobo/TOKEN')
    const reverted = revertApiEndpoint(patched, KOBO_DEFAULT_ENDPOINT)
    expect(getApiEndpoint(reverted)).toBe('https://storeapi.kobo.com')
  })
})

describe('revertApiEndpoint', () => {
  it('restores the original api_endpoint', () => {
    const conf = parseKoboConf(SAMPLE_CONF)
    const url = 'https://myserver/books/kobo/TOKEN'
    const { conf: patched, originalEndpoint } = patchApiEndpoint(conf, url)
    const reverted = revertApiEndpoint(patched, originalEndpoint)
    expect(getApiEndpoint(reverted)).toBe('https://storeapi.kobo.com')
  })

  it('preserves other keys when reverting', () => {
    const conf = parseKoboConf(SAMPLE_CONF)
    const { conf: patched, originalEndpoint } = patchApiEndpoint(
      conf,
      'https://myserver/books/kobo/TOKEN'
    )
    const reverted = revertApiEndpoint(patched, originalEndpoint)
    expect(reverted['OneStoreServices']['affiliate']).toBe('Kobo')
    expect(reverted['Version']['BuildVersion']).toBe('4.37.21586')
  })

  it('creates section if missing when reverting', () => {
    const conf = {}
    const reverted = revertApiEndpoint(conf, 'https://storeapi.kobo.com')
    expect(getApiEndpoint(reverted)).toBe('https://storeapi.kobo.com')
  })
})
