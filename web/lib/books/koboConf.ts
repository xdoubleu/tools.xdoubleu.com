type KoboSection = Record<string, string>
export type KoboConf = Record<string, KoboSection>

const TARGET_SECTION = 'OneStoreServices'
const TARGET_KEY = 'api_endpoint'

/** The stock Kobo store endpoint that ships with every Kobo device. */
export const KOBO_DEFAULT_ENDPOINT = 'https://storeapi.kobo.com'

export function parseKoboConf(raw: string): KoboConf {
  const conf: KoboConf = {}
  let section = ''

  for (const rawLine of raw.split('\n')) {
    const line = rawLine.replace(/\r$/, '').trim()

    if (line.startsWith('[') && line.endsWith(']')) {
      section = line.slice(1, -1)
      if (!conf[section]) conf[section] = {}
    } else if (section && line.includes('=')) {
      const eqIdx = line.indexOf('=')
      const key = line.slice(0, eqIdx)
      const value = line.slice(eqIdx + 1)
      conf[section][key] = value
    }
  }

  return conf
}

export function serializeKoboConf(conf: KoboConf): string {
  return Object.entries(conf)
    .map(([name, entries]) => {
      const pairs = Object.entries(entries).map(([k, v]) => `${k}=${v}`)
      return [`[${name}]`, ...pairs].join('\n')
    })
    .join('\n\n')
}

export function patchApiEndpoint(
  conf: KoboConf,
  newEndpoint: string
): { conf: KoboConf; originalEndpoint: string } {
  const section = conf[TARGET_SECTION] ?? {}
  const originalEndpoint = section[TARGET_KEY] ?? ''
  return {
    conf: {
      ...conf,
      [TARGET_SECTION]: { ...section, [TARGET_KEY]: newEndpoint }
    },
    originalEndpoint
  }
}

export function revertApiEndpoint(conf: KoboConf, originalEndpoint: string): KoboConf {
  const section = conf[TARGET_SECTION] ?? {}
  return {
    ...conf,
    [TARGET_SECTION]: { ...section, [TARGET_KEY]: originalEndpoint }
  }
}

export function getApiEndpoint(conf: KoboConf): string | undefined {
  return conf[TARGET_SECTION]?.[TARGET_KEY]
}

/**
 * Returns true when the stored api_endpoint already points at our server's
 * Kobo sync path (i.e. the device is already configured for this app).
 */
export function isManagedEndpoint(endpoint: string | undefined, apiUrl: string): boolean {
  if (!endpoint) return false
  return endpoint.startsWith(`${apiUrl}/books/kobo/`)
}
