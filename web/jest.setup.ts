import '@testing-library/jest-dom'
import { TextEncoder, TextDecoder as NodeTextDecoder } from 'util'

globalThis.TextEncoder = TextEncoder
Object.defineProperty(globalThis, 'TextDecoder', {
  value: NodeTextDecoder,
  writable: true,
  configurable: true
})
