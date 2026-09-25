/** The stock Kobo store endpoint that ships with every Kobo device. */
export const KOBO_DEFAULT_ENDPOINT = 'https://storeapi.kobo.com'

/** True when api_endpoint already points at our Kobo sync path. */
export function isManagedEndpoint(endpoint: string | undefined, apiUrl: string): boolean {
  if (!endpoint) return false
  return endpoint.startsWith(`${apiUrl}/books/kobo/`)
}
