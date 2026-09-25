/** Default device name from a serial, or a generic label without one. */
export function defaultDeviceName(serial: string): string {
  if (serial.length >= 4) {
    return `Kobo (…${serial.slice(-4)})`
  }
  return 'My Kobo'
}
