/** The stock Kobo store endpoint that ships with every Kobo device. */
export const KOBO_DEFAULT_ENDPOINT = 'https://storeapi.kobo.com'

/**
 * Returns true when the stored api_endpoint already points at our server's
 * Kobo sync path (i.e. the device is already configured for this app).
 */
export function isManagedEndpoint(endpoint: string | undefined, apiUrl: string): boolean {
  if (!endpoint) return false
  return endpoint.startsWith(`${apiUrl}/books/kobo/`)
}
