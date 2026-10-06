import {
  create,
  fromBinary,
  toBinary,
  type DescMessage,
  type DescMethodUnary,
  type MessageInitShape,
  type MessageShape
} from '@bufbuild/protobuf'
import { keepaliveTransport, transport } from '@/lib/client'

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
  /** False when `apply` already matches the server, refetching only on rejection. */
  revalidateOnSuccess?: boolean
  /** A queued write with the same key directly before this one is dropped. */
  coalesce?: (request: MessageShape<I>) => string
  /** Sends with `keepalive`, so a send started on page hide survives unload. */
  keepalive?: boolean
}

/** A write that can be queued offline; built by `defineOfflineWrite`. */
export interface OfflineWrite<I extends DescMessage = DescMessage> {
  id: string
  encode(init: MessageInitShape<I>): Uint8Array
  send(request: Uint8Array): Promise<unknown>
  apply(key: unknown, data: unknown, request: Uint8Array, hint: unknown): unknown
  describe(request: Uint8Array): string
  coalesceKey(request: Uint8Array): string | undefined
  revalidate: string
  revalidateOnSuccess: boolean
}

export function defineOfflineWrite<I extends DescMessage>(spec: WriteSpec<I>): OfflineWrite<I> {
  const { method } = spec
  const decode = (bytes: Uint8Array) => fromBinary(method.input, bytes)
  const via = spec.keepalive ? keepaliveTransport : transport
  return {
    id: `${method.parent.typeName}/${method.name}`,
    encode: (init) => toBinary(method.input, create(method.input, init)),
    send: (bytes) => via.unary(method, undefined, undefined, undefined, decode(bytes)),
    apply: (key, data, bytes, hint) => spec.apply(key, data, decode(bytes), hint),
    describe: (bytes) => spec.describe(decode(bytes)),
    coalesceKey: (bytes) => spec.coalesce?.(decode(bytes)),
    revalidate: spec.revalidate,
    revalidateOnSuccess: spec.revalidateOnSuccess ?? true
  }
}
