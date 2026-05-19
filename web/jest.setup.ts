import '@testing-library/jest-dom'
import { TextEncoder, TextDecoder } from 'util'

declare global {
  interface GlobalThis {
    TextEncoder: typeof TextEncoder
    TextDecoder: typeof TextDecoder
  }
}

globalThis.TextEncoder = TextEncoder
// eslint-disable-next-line @typescript-eslint/no-explicit-any
globalThis.TextDecoder = TextDecoder as any
