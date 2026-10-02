import {
  create,
  fromBinary,
  toBinary,
  type DescMessage,
  type DescMethodUnary,
  type MessageInitShape,
  type MessageShape
} from '@bufbuild/protobuf'
import { transport } from '@/lib/client'

export interface WriteSpec<I extends DescMessage> {
  method: DescMethodUnary<I, DescMessage>
  /**
   * Applies the write to one SWR key's cached data, returning `data` as is
   * for unrelated keys. Must be idempotent: it is re-applied over every
   * refetch while the write is queued.
   */
  apply: (key: unknown, data: unknown, request: MessageShape<I>, hint: unknown) => unknown
  /** Names the change in the "couldn't sync" list. */
  describe: (request: MessageShape<I>) => string
  /** SWR key prefix refetched once the write is sent or rejected. */
  revalidate: string
}

/** A write that can be queued offline; built by `defineOfflineWrite`. */
export interface OfflineWrite<I extends DescMessage = DescMessage> {
  id: string
  encode(init: MessageInitShape<I>): Uint8Array
  send(request: Uint8Array): Promise<unknown>
  apply(key: unknown, data: unknown, request: Uint8Array, hint: unknown): unknown
  describe(request: Uint8Array): string
  revalidate: string
}

export function defineOfflineWrite<I extends DescMessage>(spec: WriteSpec<I>): OfflineWrite<I> {
  const { method } = spec
  const decode = (bytes: Uint8Array) => fromBinary(method.input, bytes)
  return {
    id: `${method.parent.typeName}/${method.name}`,
    encode: (init) => toBinary(method.input, create(method.input, init)),
    send: (bytes) => transport.unary(method, undefined, undefined, undefined, decode(bytes)),
    apply: (key, data, bytes, hint) => spec.apply(key, data, decode(bytes), hint),
    describe: (bytes) => spec.describe(decode(bytes)),
    revalidate: spec.revalidate
  }
}
