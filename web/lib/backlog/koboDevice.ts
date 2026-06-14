/**
 * Reads the Kobo device serial number and derives a default display name.
 *
 * The `.kobo/version` file is a comma-separated line whose first field is the
 * serial number (e.g. "N418ABCD1234,4.38.21908,...").  We use the last four
 * characters as a short suffix for the default name.
 */
export async function readKoboSerial(root: FileSystemDirectoryHandle): Promise<string> {
  try {
    const koboDir = await root.getDirectoryHandle('.kobo')
    const versionHandle = await koboDir.getFileHandle('version')
    const file = await versionHandle.getFile()
    const text = (await file.text()).trim()
    const serial = text.split(',')[0].trim()
    return serial
  } catch {
    return ''
  }
}

/**
 * Derives a human-readable default device name from a serial string.
 * Falls back to a generic label when the serial is not available.
 */
export function defaultDeviceName(serial: string): string {
  if (serial.length >= 4) {
    return `Kobo (…${serial.slice(-4)})`
  }
  return 'My Kobo'
}
