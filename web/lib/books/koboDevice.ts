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
