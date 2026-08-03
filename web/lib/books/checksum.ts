/**
 * Computes the SHA-256 hex digest of a File's content using the Web Crypto API.
 * The result matches Go's fmt.Sprintf("%x", sha256.Sum256(data)) format
 * (lowercase hex, no separators).
 */
export async function sha256Hex(file: File): Promise<string> {
  const buffer = await file.arrayBuffer()
  const hashBuffer = await crypto.subtle.digest('SHA-256', buffer)
  const bytes = new Uint8Array(hashBuffer)
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
}
